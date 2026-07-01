package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backend/internal/model/entity"

	"github.com/go-redis/redis/v8"
)

// FlashCache 秒杀Redis缓存操作接口
type FlashCache interface {
	// === 排队入场 ===
	IncrQueueCount(flashSaleID uint) (int64, error)           // 原子递增排队计数，返回当前序号
	DecrQueueCount(flashSaleID uint) (int64, error)           // 原子递减（入场被拒时回滚）
	AddAdmittedUser(flashSaleID uint, userID uint) error      // 将用户添加到入场集合
	RemoveAdmittedUser(flashSaleID uint, userID uint) error  // 移除用户入场资格（超时释放时调用）
	IsUserAdmitted(flashSaleID uint, userID uint) (bool, error) // 检查用户是否已入场
	GetQueueCount(flashSaleID uint) (int64, error)            // 获取当前排队人数

	// === 库存扣减 ===
	AtomicDeductStock(flashSaleID uint, userID uint) (int64, int64, error) // Lua原子扣减：返回(code, remaining)
	RollbackDeduct(flashSaleID uint, userID uint) error                    // 回滚扣减（DB写入失败时调用）
	GetRemainingStock(flashSaleID uint) (int64, error)                      // 查询剩余库存
	StockKeyExists(flashSaleID uint) (bool, error)                          // 检查库存Key是否存在（区分"Key不存在"和"库存为0"）
	SetFlashStock(flashSaleID uint, stock int64) error                     // 设置剩余库存（对账修正用）
	AtomicSetStock(flashSaleID uint, expected int64, newVal int64) (int64, int64, error) // Lua原子CAS设置库存（对账修正用）
	GetDeductTrackUsers(flashSaleID uint) ([]string, error)                             // 获取已购追踪列表（崩溃恢复短期窗口用）
	SetFlashSoldOut(flashSaleID uint) error                                // 设置全局售罄标记（多实例共享）
	IsFlashSoldOut(flashSaleID uint) (bool, error)                         // 检查全局售罄标记
	DeleteFlashSoldOut(flashSaleID uint) error                             // 删除售罄标记（库存释放时调用）
	GetRandomPurchasedUsers(flashSaleID uint, count int64) ([]string, error) // 从已购集合随机抽样用户（幽灵用户检测用）
	DeleteFlashInfo(flashSaleID uint) error                                  // 删除活动信息缓存（修改活动时调用）

	// === 初始化与预热 ===
	WarmUpStock(flashSaleID uint, stock int) error // 将秒杀库存预热到Redis
	ClearFlashCache(flashSaleID uint) error        // 清理秒杀相关所有缓存

	// === 活动信息缓存 ===
	SetFlashInfo(flashSaleID uint, info map[string]interface{}) error // 缓存活动基本信息
	GetFlashInfo(flashSaleID uint) (map[string]string, error)         // 获取活动缓存

	// === 防丢失记录（崩溃恢复） ===
	SetPendingOrder(flashSaleID uint, userID uint, orderNo string) error // 记录待处理订单
	GetPendingOrders(flashSaleID uint) (map[string]string, error)        // 获取所有待处理记录
	RemovePendingOrder(flashSaleID uint, userID uint) error              // 清除单条待处理记录
	MarkPendingRolledBack(flashSaleID uint, userID uint) error                            // 标记防丢失记录为已回滚（防止误重建）
	ScanAllPendingKeys() ([]string, error)                               // 扫描所有 flash:pending:* 的key
	CleanPendingKey(key string) error                                    // 清理整个 pending key

	// === 验证码 ===
	SetCaptcha(captchaID, answer string, ttl time.Duration) error          // 存入验证码答案
	GetAndDeleteCaptcha(captchaID string) (string, error)                  // 原子获取并删除验证码
}

type flashCache struct {
	client *redis.Client
	ctx    context.Context
}

// NewFlashCache 创建秒杀缓存实例（供 Wire 注入）
func NewFlashCache(client *redis.Client) FlashCache {
	return &flashCache{
		client: client,
		ctx:    context.Background(),
	}
}

// ==================== Lua 原子扣库存脚本 ====================
var atomicDeductScript = redis.NewScript(`
	local stock_key = KEYS[1]
	local user_key = KEYS[2]
	local track_key = KEYS[3]       -- 已购追踪列表（用于崩溃恢复扫描）
	local user_id = ARGV[1]

	local stock = redis.call('GET', stock_key)
	if stock == false then
		return {3, 0}  -- 活动未预热，库存key不存在
	end

	stock = tonumber(stock)
	if stock <= 0 then
		return {1, 0}  -- 库存不足
	end

	if redis.call('SISMEMBER', user_key, user_id) == 1 then
		return {2, 0}  -- 已抢购过
	end

	redis.call('DECR', stock_key)
	redis.call('SADD', user_key, user_id)
	redis.call('EXPIRE', user_key, 172800)  -- 48h TTL，防止异常退出后 Set 永久残留
	-- 追加到已购追踪列表，TTL 5分钟（短期恢复窗口，超时后由 pending 机制兜底）
	redis.call('RPUSH', track_key, user_id)
	redis.call('EXPIRE', track_key, 300)

	return {0, stock - 1}  -- 成功，返回剩余库存
`)

// ==================== Lua 原子回滚脚本 ====================
var atomicRollbackScript = redis.NewScript(`
	local stock_key = KEYS[1]
	local user_key = KEYS[2]
	local user_id = ARGV[1]

	local is_member = redis.call('SISMEMBER', user_key, user_id)
	if is_member == 0 then
		return {0, 0}
	end

	redis.call('INCR', stock_key)
	redis.call('SREM', user_key, user_id)

	local stock = redis.call('GET', stock_key)
	return {1, stock}
`)

// ==================== Lua 原子CAS设置库存脚本（对账修正用，防止并发覆盖） ====================
var atomicSetStockScript = redis.NewScript(`
	local stock_key = KEYS[1]
	local expected = tonumber(ARGV[1])
	local new_val = tonumber(ARGV[2])

	local current = redis.call('GET', stock_key)
	if current == false then
		-- Key 不存在（从未预热或 Redis 数据丢失），无并发扣减可冲突，直接创建
		redis.call('SET', stock_key, new_val)
		return {1, new_val}
	end
	current = tonumber(current)
	if current == expected then
		redis.call('SET', stock_key, new_val)
		return {1, new_val}
	else
		return {0, current}
	end
`)


// ==================== 排队入场 ====================

// queueKey 生成排队计数器的Redis键名
func queueKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:queue:%d", flashSaleID)
}

// admittedKey 生成入场用户键名（每个用户独立键，自带TTL自动过期）
func admittedKey(flashSaleID uint, userID uint) string {
	return fmt.Sprintf("flash:admitted:%d:%d", flashSaleID, userID)
}

// IncrQueueCount 原子递增排队计数，返回当前排队序号
func (c *flashCache) IncrQueueCount(flashSaleID uint) (int64, error) {
	key := queueKey(flashSaleID)
	count, err := c.client.Incr(c.ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("排队计数递增失败: %w", err)
	}
	// 首次创建时设置 2 小时过期，防止活动异常结束时 key 永久残留
	if count == 1 {
		c.client.Expire(c.ctx, key, 2*time.Hour)
	}
	return count, nil
}

// DecrQueueCount 原子递减排队计数（入场被拒时释放名额）
// 若 Key 不存在则直接返回 0，避免 Redis DECR 自动创建值为 -1 的 Key
func (c *flashCache) DecrQueueCount(flashSaleID uint) (int64, error) {
	key := queueKey(flashSaleID)
	exists, err := c.client.Exists(c.ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, nil
	}
	count, err := c.client.Decr(c.ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("排队计数递减失败: %w", err)
	}
	return count, nil
}

// AddAdmittedUser 将用户标记为已入场，独立键自带5分钟TTL自动过期
func (c *flashCache) AddAdmittedUser(flashSaleID uint, userID uint) error {
	return c.client.Set(c.ctx, admittedKey(flashSaleID, userID), "1", 5*time.Minute).Err()
}

// IsUserAdmitted 检查用户是否已获得入场资格
func (c *flashCache) IsUserAdmitted(flashSaleID uint, userID uint) (bool, error) {
	exists, err := c.client.Exists(c.ctx, admittedKey(flashSaleID, userID)).Result()
	return exists == 1, err
}

// RemoveAdmittedUser 移除用户入场资格（超时释放库存时调用）
func (c *flashCache) RemoveAdmittedUser(flashSaleID uint, userID uint) error {
	return c.client.Del(c.ctx, admittedKey(flashSaleID, userID)).Err()
}

// GetQueueCount 获取当前排队人数
func (c *flashCache) GetQueueCount(flashSaleID uint) (int64, error) {
	val, err := c.client.Get(c.ctx, queueKey(flashSaleID)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// ==================== 库存扣减 ====================

// stockKey 生成库存Redis键名
func stockKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:stock:%d", flashSaleID)
}

// userKey 生成已抢购用户集合Redis键名
func userSetKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:user:%d", flashSaleID)
}

// deductTrackKey 生成已购追踪列表键名（Lua脚本中用于崩溃恢复短期窗口）
func deductTrackKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:deduct_track:%d", flashSaleID)
}

// AtomicDeductStock 使用Lua脚本原子扣减库存
// 返回值: (code, remaining)
//
//	code=0 成功 | code=1 库存不足 | code=2 已抢过 | code=3 活动未预热
func (c *flashCache) AtomicDeductStock(flashSaleID uint, userID uint) (int64, int64, error) {
	keys := []string{stockKey(flashSaleID), userSetKey(flashSaleID), deductTrackKey(flashSaleID)}
	args := []interface{}{userID}

	result, err := atomicDeductScript.Run(c.ctx, c.client, keys, args...).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("Lua脚本执行失败: %w", err)
	}

	// Lua返回的是 []interface{}
	vals, ok := result.([]interface{})
	if !ok || len(vals) != 2 {
		return 0, 0, fmt.Errorf("Lua脚本返回格式异常: %v", result)
	}

	code, ok := vals[0].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("Lua脚本code类型断言失败: got %T", vals[0])
	}
	remaining, ok := vals[1].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("Lua脚本remaining类型断言失败: got %T", vals[1])
	}
	return code, remaining, nil
}

// RollbackDeduct 回滚扣减：恢复库存 + 移除用户资格
func (c *flashCache) RollbackDeduct(flashSaleID uint, userID uint) error {
	keys := []string{stockKey(flashSaleID), userSetKey(flashSaleID)}
	args := []interface{}{userID}
	result, err := atomicRollbackScript.Run(c.ctx, c.client, keys, args...).Result()
	if err != nil {
		return fmt.Errorf("回滚库存失败(Lua): %w", err)
	}
	vals, ok := result.([]interface{})
	if !ok {
		return fmt.Errorf("回滚脚本返回格式异常: %v", result)
	}
	code, _ := vals[0].(int64)
	if code == 0 {
		return fmt.Errorf("用户 %d 不在活动 %d 的已购集合中", userID, flashSaleID)
	}
	return nil
}

// GetRemainingStock 查询Redis中剩余秒杀库存
func (c *flashCache) GetRemainingStock(flashSaleID uint) (int64, error) {
	val, err := c.client.Get(c.ctx, stockKey(flashSaleID)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// StockKeyExists 检查库存Key是否存在于Redis中
// 用于区分"Key从未创建（未预热）"和"库存已售罄（值为0）"两种状态
func (c *flashCache) StockKeyExists(flashSaleID uint) (bool, error) {
	count, err := c.client.Exists(c.ctx, stockKey(flashSaleID)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetFlashStock 设置Redis 中剩余秒杀库存（对账修正用）
func (c *flashCache) SetFlashStock(flashSaleID uint, stock int64) error {
	return c.client.Set(c.ctx, stockKey(flashSaleID), stock, 48*time.Hour).Err()
}

// AtomicSetStock 使用Lua CAS脚本原子设置库存（对账修正用，防止并发覆盖）
// 仅当 current == expected 时才执行 SET，否则放弃修正
// 返回值: (code, actualStock) — code=1 成功, code=0 冲突(放弃修正)
func (c *flashCache) AtomicSetStock(flashSaleID uint, expected int64, newVal int64) (int64, int64, error) {
	keys := []string{stockKey(flashSaleID)}
	args := []interface{}{expected, newVal}

	result, err := atomicSetStockScript.Run(c.ctx, c.client, keys, args...).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("CAS设置库存 Lua脚本执行失败: %w", err)
	}

	arr, ok := result.([]interface{})
	if !ok || len(arr) < 2 {
		return 0, 0, fmt.Errorf("CAS脚本返回值格式错误: %v", result)
	}
	code, ok := arr[0].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("CAS脚本code类型断言失败: got %T", arr[0])
	}
	val, ok := arr[1].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("CAS脚本val类型断言失败: got %T", arr[1])
	}
	return code, val, nil
}

// GetRandomPurchasedUsers 从已购集合中随机抽取用户（用于幽灵用户检测）
func (c *flashCache) GetRandomPurchasedUsers(flashSaleID uint, count int64) ([]string, error) {
	return c.client.SRandMemberN(c.ctx, userSetKey(flashSaleID), count).Result()
}

// DeleteFlashInfo 删除活动信息缓存（修改活动时主动失效）
func (c *flashCache) DeleteFlashInfo(flashSaleID uint) error {
	return c.client.Del(c.ctx, fmt.Sprintf("flash:info:%d", flashSaleID)).Err()
}

// GetDeductTrackUsers 获取已购追踪列表中的所有用户ID（用于崩溃恢复短期窗口扫描）
// 该列表由 Lua 扣减脚本在 SADD 用户集合时同步写入，TTL 5分钟
func (c *flashCache) GetDeductTrackUsers(flashSaleID uint) ([]string, error) {
	return c.client.LRange(c.ctx, deductTrackKey(flashSaleID), 0, -1).Result()
}

// pendingStatusConfirmed 和 pendingStatusRolledBack 定义防丢失记录的状态值
const (
	pendingStatusConfirmed  = "confirmed"   // 正常扣减成功，等待DB写入
	pendingStatusRolledBack = "rolled_back" // Redis已回滚，记录仅用于防止误重建
)

// formatPendingValue 将 orderNo 和 status 编码为存储格式
func formatPendingValue(orderNo, status string) string {
	return orderNo + "|" + status
}

// parsePendingValue 解析存储格式，返回 (orderNo, status)
// 兼容旧格式（无状态字段），旧格式默认视为 confirmed
func parsePendingValue(raw string) (orderNo, status string) {
	parts := strings.SplitN(raw, "|", 2)
	orderNo = parts[0]
	if len(parts) == 2 {
		status = parts[1]
	} else {
		status = pendingStatusConfirmed // 旧格式兼容
	}
	return
}

// MarkPendingRolledBack 将防丢失记录标记为已回滚状态（防止 RecoverPendingOrders 误重建订单）
// 在 RollbackDeduct 成功后调用，替代直接删除 pending 记录
func (c *flashCache) MarkPendingRolledBack(flashSaleID uint, userID uint) error {
	key := pendingKey(flashSaleID)
	field := strconv.Itoa(int(userID))
	// 读取现有值，提取 orderNo，重新编码为 rolled_back 状态
	existing, err := c.client.HGet(c.ctx, key, field).Result()
	if err != nil {
		return fmt.Errorf("读取防丢失记录失败: %w", err)
	}
	orderNo, _ := parsePendingValue(existing)
	newValue := formatPendingValue(orderNo, pendingStatusRolledBack)
	return c.client.HSet(c.ctx, key, field, newValue).Err()
}



// soldOutKey 生成全局售罄标记的 Redis 键名（多实例共享）
func soldOutKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:soldout:%d", flashSaleID)
}

// SetFlashSoldOut 设置全局售罄标记（TTL 5分钟，防止 key 永久残留）
func (c *flashCache) SetFlashSoldOut(flashSaleID uint) error {
	return c.client.Set(c.ctx, soldOutKey(flashSaleID), "1", 5*time.Minute).Err()
}

// IsFlashSoldOut 检查全局售罄标记是否已设置
func (c *flashCache) IsFlashSoldOut(flashSaleID uint) (bool, error) {
	_, err := c.client.Get(c.ctx, soldOutKey(flashSaleID)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// DeleteFlashSoldOut 删除全局售罄标记（库存释放时调用）
func (c *flashCache) DeleteFlashSoldOut(flashSaleID uint) error {
	return c.client.Del(c.ctx, soldOutKey(flashSaleID)).Err()
}

// ==================== 初始化与预热 ====================

// WarmUpStock 将秒杀库存加载到Redis
func (c *flashCache) WarmUpStock(flashSaleID uint, stock int) error {
	key := stockKey(flashSaleID)
	// 设置 48 小时过期（防止活动异常结束时 key 永久残留），足够覆盖任何合理秒杀时长
	if err := c.client.Set(c.ctx, key, stock, 48*time.Hour).Err(); err != nil {
		return fmt.Errorf("预热库存失败: %w", err)
	}
	// 清空上一轮的已抢购用户集合
	c.client.Del(c.ctx, userSetKey(flashSaleID))
	// 清空排队计数器
	c.client.Del(c.ctx, queueKey(flashSaleID))
	// 清空入场用户键（SCAN 删除单用户键）
	c.cleanAdmittedKeys(flashSaleID)
	return nil
}

// ClearFlashCache 清理秒杀相关的所有Redis缓存
func (c *flashCache) ClearFlashCache(flashSaleID uint) error {
	c.cleanAdmittedKeys(flashSaleID) // 清理所有单用户入场键
	keys := []string{
		stockKey(flashSaleID),
		userSetKey(flashSaleID),
		queueKey(flashSaleID),
		fmt.Sprintf("flash:info:%d", flashSaleID),
		pendingKey(flashSaleID),
		soldOutKey(flashSaleID),
	}
	return c.client.Del(c.ctx, keys...).Err()
}

// cleanAdmittedKeys 扫描并删除指定活动的所有单用户入场键
func (c *flashCache) cleanAdmittedKeys(flashSaleID uint) {
	pattern := fmt.Sprintf("flash:admitted:%d:*", flashSaleID)
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(c.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return
		}
		if len(keys) > 0 {
			c.client.Del(c.ctx, keys...)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

// ==================== 活动信息缓存 ====================

// SetFlashInfo 缓存秒杀活动基本信息
func (c *flashCache) SetFlashInfo(flashSaleID uint, info map[string]interface{}) error {
	key := fmt.Sprintf("flash:info:%d", flashSaleID)
	if err := c.client.HMSet(c.ctx, key, info).Err(); err != nil {
		return fmt.Errorf("缓存活动信息失败: %w", err)
	}
	// 10分钟自动过期
	c.client.Expire(c.ctx, key, 10*time.Minute)
	return nil
}

// GetFlashInfo 获取缓存的秒杀活动信息
func (c *flashCache) GetFlashInfo(flashSaleID uint) (map[string]string, error) {
	key := fmt.Sprintf("flash:info:%d", flashSaleID)
	return c.client.HGetAll(c.ctx, key).Result()
}

// ==================== 防丢失记录（崩溃恢复） ====================

// pendingKey 生成防丢失记录的Redis键名
func pendingKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:pending:%d", flashSaleID)
}

// SetPendingOrder 记录待处理的秒杀订单（Redis扣减成功但DB尚未写入时调用）
// 存储格式: "orderNo|confirmed"，支持 RecoverPendingOrders 按状态区分处理
func (c *flashCache) SetPendingOrder(flashSaleID uint, userID uint, orderNo string) error {
	key := pendingKey(flashSaleID)
	value := formatPendingValue(orderNo, pendingStatusConfirmed)
	if err := c.client.HSet(c.ctx, key, strconv.Itoa(int(userID)), value).Err(); err != nil {
		return fmt.Errorf("设置防丢失记录失败: %w", err)
	}
	// 使用独立的 Expire 命令设置过期时间（非原子操作，但风险可控：
	// 若HSET与Expire之间崩溃，pending key无TTL；下次重启RecoverPendingOrders会检测到订单已存在，仅清理Redis记录）
	if err := c.client.Expire(c.ctx, key, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("设置防丢失记录过期时间失败: %w", err)
	}
	return nil
}

// GetPendingOrders 获取指定活动的所有待处理订单
func (c *flashCache) GetPendingOrders(flashSaleID uint) (map[string]string, error) {
	return c.client.HGetAll(c.ctx, pendingKey(flashSaleID)).Result()
}

// RemovePendingOrder 删除单条待处理记录（DB写入成功后调用）
func (c *flashCache) RemovePendingOrder(flashSaleID uint, userID uint) error {
	return c.client.HDel(c.ctx, pendingKey(flashSaleID), strconv.Itoa(int(userID))).Err()
}

// ScanAllPendingKeys 扫描所有 flash:pending:* 键（崩溃恢复时用）
func (c *flashCache) ScanAllPendingKeys() ([]string, error) {
	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = c.client.Scan(c.ctx, cursor, "flash:pending:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("扫描防丢失记录失败: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

// CleanPendingKey 删除整个 pending key
func (c *flashCache) CleanPendingKey(key string) error {
	return c.client.Del(c.ctx, key).Err()
}

// ==================== 验证码 ====================

// captchaKey 生成验证码 Redis 键名
func captchaKey(captchaID string) string {
	return fmt.Sprintf("captcha:%s", captchaID)
}

// SetCaptcha 存储验证码答案，ttl 后自动过期
func (c *flashCache) SetCaptcha(captchaID, answer string, ttl time.Duration) error {
	return c.client.Set(c.ctx, captchaKey(captchaID), answer, ttl).Err()
}

// ==================== Lua 原子获取并删除验证码脚本 ====================
var getAndDeleteCaptchaScript = redis.NewScript(`
	local key = KEYS[1]
	local answer = redis.call('GET', key)
	if answer == false then
		return ""
	end
	redis.call('DEL', key)
	return answer
`)

// GetAndDeleteCaptcha 原子操作：获取并立即删除（Lua保证GET+DEL原子性，防并发重复使用）
func (c *flashCache) GetAndDeleteCaptcha(captchaID string) (string, error) {
	key := captchaKey(captchaID)
	result, err := getAndDeleteCaptchaScript.Run(c.ctx, c.client, []string{key}).Result()
	if err != nil {
		return "", fmt.Errorf("验证码操作失败: %w", err)
	}
	answer, ok := result.(string)
	if !ok {
		return "", nil // key不存在时Lua返回空字符串
	}
	return answer, nil
}

// ==================== 辅助序列化方法 ====================

// MarshalFlashInfo 将FlashSale实体序列化为缓存用的map
func MarshalFlashInfo(flash *entity.FlashSale) map[string]interface{} {
	return map[string]interface{}{
		"product_id":   flash.ProductID,
		"product_name": flash.Product.Name,
		"flash_price":  flash.FlashPrice,
		"flash_stock":  flash.FlashStock,
		"start_time":   flash.StartTime.Format("2006-01-02 15:04:05"),
		"end_time":     flash.EndTime.Format("2006-01-02 15:04:05"),
		"status":       flash.Status,
		"image":        flash.Product.Image,
	}
}

// UnmarshalFlashInfo 从缓存map反序列化
func UnmarshalFlashInfo(data map[string]string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}
	// 解析数值字段
	if v, ok := data["product_id"]; ok {
		id, _ := strconv.ParseUint(v, 10, 64)
		result["product_id"] = uint(id)
	}
	if v, ok := data["flash_price"]; ok {
		f, _ := strconv.ParseFloat(v, 64)
		result["flash_price"] = f
	}
	if v, ok := data["flash_stock"]; ok {
		i, _ := strconv.Atoi(v)
		result["flash_stock"] = i
	}
	if v, ok := data["status"]; ok {
		i, _ := strconv.Atoi(v)
		result["status"] = i
	}
	return result, nil
}

// 确保 json 包被使用（MarshalFlashInfo中未直接用到，但保留导入以备扩展）
var _ = json.Marshal
