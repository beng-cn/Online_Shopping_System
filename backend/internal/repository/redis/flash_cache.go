package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"backend/internal/model/entity"

	"github.com/go-redis/redis/v8"
)

// FlashCache 秒杀Redis缓存操作接口
type FlashCache interface {
	// === 排队入场 ===
	IncrQueueCount(flashSaleID uint) (int64, error)           // 原子递增排队计数，返回当前序号
	AddAdmittedUser(flashSaleID uint, userID uint) error      // 将用户添加到入场集合
	IsUserAdmitted(flashSaleID uint, userID uint) (bool, error) // 检查用户是否已入场
	GetQueueCount(flashSaleID uint) (int64, error)            // 获取当前排队人数

	// === 库存扣减 ===
	AtomicDeductStock(flashSaleID uint, userID uint) (int64, int64, error) // Lua原子扣减：返回(code, remaining)
	RollbackDeduct(flashSaleID uint, userID uint) error                    // 回滚扣减（DB写入失败时调用）
	GetRemainingStock(flashSaleID uint) (int64, error)                      // 查询剩余库存

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
	ScanAllPendingKeys() ([]string, error)                               // 扫描所有 flash:pending:* 的key
	CleanPendingKey(key string) error                                    // 清理整个 pending key
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

	return {0, stock - 1}  -- 成功，返回剩余库存
`)

// ==================== 排队入场 ====================

// queueKey 生成排队计数器的Redis键名
func queueKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:queue:%d", flashSaleID)
}

// admittedKey 生成入场用户集合的Redis键名
func admittedKey(flashSaleID uint) string {
	return fmt.Sprintf("flash:queue:%d:admitted", flashSaleID)
}

// IncrQueueCount 原子递增排队计数，返回当前排队序号
func (c *flashCache) IncrQueueCount(flashSaleID uint) (int64, error) {
	count, err := c.client.Incr(c.ctx, queueKey(flashSaleID)).Result()
	if err != nil {
		return 0, fmt.Errorf("排队计数递增失败: %w", err)
	}
	return count, nil
}

// AddAdmittedUser 将用户ID添加到已入场集合
func (c *flashCache) AddAdmittedUser(flashSaleID uint, userID uint) error {
	key := admittedKey(flashSaleID)
	if err := c.client.SAdd(c.ctx, key, userID).Err(); err != nil {
		return fmt.Errorf("添加入场用户失败: %w", err)
	}
	// 设置5分钟过期，防止内存泄漏
	c.client.Expire(c.ctx, key, 5*time.Minute)
	return nil
}

// IsUserAdmitted 检查用户是否已获得入场资格
func (c *flashCache) IsUserAdmitted(flashSaleID uint, userID uint) (bool, error) {
	return c.client.SIsMember(c.ctx, admittedKey(flashSaleID), userID).Result()
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

// AtomicDeductStock 使用Lua脚本原子扣减库存
// 返回值: (code, remaining)
//
//	code=0 成功 | code=1 库存不足 | code=2 已抢过 | code=3 活动未预热
func (c *flashCache) AtomicDeductStock(flashSaleID uint, userID uint) (int64, int64, error) {
	keys := []string{stockKey(flashSaleID), userSetKey(flashSaleID)}
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

	code, _ := vals[0].(int64)
	remaining, _ := vals[1].(int64)
	return code, remaining, nil
}

// RollbackDeduct 回滚扣减：恢复库存 + 移除用户资格
func (c *flashCache) RollbackDeduct(flashSaleID uint, userID uint) error {
	pipe := c.client.Pipeline()
	pipe.Incr(c.ctx, stockKey(flashSaleID))
	pipe.SRem(c.ctx, userSetKey(flashSaleID), userID)
	_, err := pipe.Exec(c.ctx)
	if err != nil {
		return fmt.Errorf("回滚库存失败: %w", err)
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

// ==================== 初始化与预热 ====================

// WarmUpStock 将秒杀库存加载到Redis
func (c *flashCache) WarmUpStock(flashSaleID uint, stock int) error {
	key := stockKey(flashSaleID)
	if err := c.client.Set(c.ctx, key, stock, 0).Err(); err != nil {
		return fmt.Errorf("预热库存失败: %w", err)
	}
	// 清空上一轮的已抢购用户集合
	c.client.Del(c.ctx, userSetKey(flashSaleID))
	// 清空排队计数器
	c.client.Del(c.ctx, queueKey(flashSaleID))
	c.client.Del(c.ctx, admittedKey(flashSaleID))
	return nil
}

// ClearFlashCache 清理秒杀相关的所有Redis缓存
func (c *flashCache) ClearFlashCache(flashSaleID uint) error {
	keys := []string{
		stockKey(flashSaleID),
		userSetKey(flashSaleID),
		queueKey(flashSaleID),
		admittedKey(flashSaleID),
		fmt.Sprintf("flash:info:%d", flashSaleID),
	}
	return c.client.Del(c.ctx, keys...).Err()
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
func (c *flashCache) SetPendingOrder(flashSaleID uint, userID uint, orderNo string) error {
	key := pendingKey(flashSaleID)
	if err := c.client.HSet(c.ctx, key, strconv.Itoa(int(userID)), orderNo).Err(); err != nil {
		return fmt.Errorf("设置防丢失记录失败: %w", err)
	}
	// 10分钟过期，超过此时间认为恢复无望
	c.client.Expire(c.ctx, key, 10*time.Minute)
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
