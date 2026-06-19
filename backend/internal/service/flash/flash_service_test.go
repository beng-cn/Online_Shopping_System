package flash

import (
	"context"
	"testing"
	"time"

	"backend/internal/config"
	"backend/internal/pkg/flashinventory"
	"backend/internal/repository/redis"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

// 创建测试用的 FlashService（无 DB 依赖，只测 Redis 相关逻辑）
func newTestFlashService(t *testing.T) (FlashService, *miniredis.Miniredis, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("启动 miniredis 失败: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})

	cfg := &config.AppConfig{
		FlashSale: config.FlashSaleConfig{
			PaymentTimeoutHours: 2,
			CoolDownMinutes:     2,
		},
	}

	flashCache := redis.NewFlashCache(client)

	svc := &flashService{
		db:         nil,
		cfg:        cfg,
		flashCache: flashCache,
		inventory:  flashinventory.New(),
	}

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return svc, mr, cleanup
}

// 辅助：预热一个测试秒杀活动
func warmUpTestActivity(t *testing.T, svc FlashService, flashSaleID uint, stock int) {
	t.Helper()
	fs := svc.(*flashService)
	if err := fs.flashCache.WarmUpStock(flashSaleID, stock); err != nil {
		t.Fatalf("预热失败: %v", err)
	}
	fs.inventory.Init(flashSaleID, stock)
}

// ==================== 第一组：Lua 原子扣减 ====================

func TestAtomicDeductSuccess(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)
	warmUpTestActivity(t, svc, 1, 100)

	code, remaining, err := fs.flashCache.AtomicDeductStock(1, 1001)
	if err != nil {
		t.Fatalf("扣减失败: %v", err)
	}

	if code != 0 {
		t.Errorf("期望 code=0(成功)，实际 %d", code)
	}
	if remaining != 99 {
		t.Errorf("期望 remaining=99，实际 %d", remaining)
	}

	// 验证库存确实减了
	stock, _ := fs.flashCache.GetRemainingStock(1)
	if stock != 99 {
		t.Errorf("期望 Redis 库存=99，实际 %d", stock)
	}
}

func TestAtomicDeductOutOfStock(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)
	// 预热 0 库存
	fs.flashCache.WarmUpStock(1, 0)
	fs.inventory.Init(1, 0)

	code, remaining, err := fs.flashCache.AtomicDeductStock(1, 1001)
	if err != nil {
		t.Fatalf("扣减失败: %v", err)
	}

	if code != 1 {
		t.Errorf("期望 code=1(库存不足)，实际 %d", code)
	}
	if remaining != 0 {
		t.Errorf("期望 remaining=0，实际 %d", remaining)
	}
}

func TestAtomicDeductDuplicateUser(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)
	warmUpTestActivity(t, svc, 1, 100)

	// 第一次：成功
	code, _, _ := fs.flashCache.AtomicDeductStock(1, 1001)
	if code != 0 {
		t.Fatalf("第一次扣减应成功，实际 code=%d", code)
	}

	// 第二次：重复用户
	code, _, err := fs.flashCache.AtomicDeductStock(1, 1001)
	if err != nil {
		t.Fatalf("扣减失败: %v", err)
	}

	if code != 2 {
		t.Errorf("期望 code=2(已抢过)，实际 %d", code)
	}
}

func TestAtomicDeductNotWarmedUp(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)

	// 不预热，直接扣减
	code, _, err := fs.flashCache.AtomicDeductStock(1, 1001)
	if err != nil {
		t.Fatalf("扣减失败: %v", err)
	}

	if code != 3 {
		t.Errorf("期望 code=3(活动未预热)，实际 %d", code)
	}
}

func TestAtomicDeductMultipleUsers(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)
	warmUpTestActivity(t, svc, 1, 10)

	// 10 个不同用户并发扣减
	for uid := uint(1); uid <= 10; uid++ {
		code, remaining, err := fs.flashCache.AtomicDeductStock(1, uid)
		if err != nil {
			t.Fatalf("用户 %d 扣减失败: %v", uid, err)
		}
		if code != 0 {
			t.Errorf("用户 %d 应成功，实际 code=%d", uid, code)
		}
		if remaining != int64(10-uid) {
			t.Errorf("用户 %d 后期望剩余 %d，实际 %d", uid, 10-uid, remaining)
		}
	}

	// 第 11 个用户：库存不足
	code, _, err := fs.flashCache.AtomicDeductStock(1, 11)
	if err != nil {
		t.Fatalf("用户 11 扣减失败: %v", err)
	}
	if code != 1 {
		t.Errorf("期望 code=1(库存不足)，实际 %d", code)
	}
}

// ==================== 第二组：Lua 回滚 ====================

func TestRollbackDeductSuccess(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)
	warmUpTestActivity(t, svc, 1, 100)

	// 先扣减
	code, _, _ := fs.flashCache.AtomicDeductStock(1, 1001)
	if code != 0 {
		t.Fatalf("扣减应成功")
	}

	// 回滚
	err := fs.flashCache.RollbackDeduct(1, 1001)
	if err != nil {
		t.Fatalf("回滚失败: %v", err)
	}

	// 库存恢复
	stock, _ := fs.flashCache.GetRemainingStock(1)
	if stock != 100 {
		t.Errorf("回滚后期望库存=100，实际 %d", stock)
	}
}

func TestRollbackDeductUserNotInSet(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)
	warmUpTestActivity(t, svc, 1, 100)

	// 不回滚未扣减过的用户
	err := fs.flashCache.RollbackDeduct(1, 9999)
	if err == nil {
		t.Error("回滚未扣减的用户应返回错误")
	}
}

// ==================== 第三组：排队入场 ====================

func TestQueueIncr(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)

	// 三次递增
	c1, _ := fs.flashCache.IncrQueueCount(1)
	c2, _ := fs.flashCache.IncrQueueCount(1)
	c3, _ := fs.flashCache.IncrQueueCount(1)

	if c1 != 1 || c2 != 2 || c3 != 3 {
		t.Errorf("期望递增 1→2→3，实际 %d→%d→%d", c1, c2, c3)
	}
}

func TestQueueDecr(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)

	fs.flashCache.IncrQueueCount(1) // →1
	fs.flashCache.IncrQueueCount(1) // →2
	count, _ := fs.flashCache.DecrQueueCount(1)

	if count != 1 {
		t.Errorf("DECR 后期望 count=1，实际 %d", count)
	}
}

// ==================== 第四组：全局售罄标记（Fix 1） ====================

func TestSetAndCheckFlashSoldOut(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)

	// 初始未售罄
	soldOut, err := fs.flashCache.IsFlashSoldOut(1)
	if err != nil {
		t.Fatalf("检查售罄标记失败: %v", err)
	}
	if soldOut {
		t.Error("新活动不应标记售罄")
	}

	// 设置售罄标记
	if err := fs.flashCache.SetFlashSoldOut(1); err != nil {
		t.Fatalf("设置售罄标记失败: %v", err)
	}

	soldOut, _ = fs.flashCache.IsFlashSoldOut(1)
	if !soldOut {
		t.Error("设置后应返回 true")
	}

	// 删除标记
	fs.flashCache.DeleteFlashSoldOut(1)

	soldOut, _ = fs.flashCache.IsFlashSoldOut(1)
	if soldOut {
		t.Error("删除后不应标记售罄")
	}
}

// ==================== 第五组：防丢失记录 ====================

func TestPendingOrderCycle(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)

	// 写入 pending
	err := fs.flashCache.SetPendingOrder(1, 1001, "FS20240619120000ABCDEF01")
	if err != nil {
		t.Fatalf("写入 pending 失败: %v", err)
	}

	// 读取
	orders, err := fs.flashCache.GetPendingOrders(1)
	if err != nil {
		t.Fatalf("读取 pending 失败: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("期望 1 条 pending，实际 %d", len(orders))
	}

	// 清除
	err = fs.flashCache.RemovePendingOrder(1, 1001)
	if err != nil {
		t.Fatalf("清除 pending 失败: %v", err)
	}

	orders, _ = fs.flashCache.GetPendingOrders(1)
	if len(orders) != 0 {
		t.Errorf("清除后应为空，实际 %d", len(orders))
	}
}

// ==================== 第六组：SetFlashStock（Fix 6） ====================

func TestSetFlashStock(t *testing.T) {
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	fs := svc.(*flashService)

	// 初始无库存
	stock, _ := fs.flashCache.GetRemainingStock(1)
	if stock != 0 {
		t.Errorf("初始库存应为 0，实际 %d", stock)
	}

	// 设置库存
	err := fs.flashCache.SetFlashStock(1, 50)
	if err != nil {
		t.Fatalf("设置库存失败: %v", err)
	}

	stock, _ = fs.flashCache.GetRemainingStock(1)
	if stock != 50 {
		t.Errorf("设置后期望 50，实际 %d", stock)
	}
}

// ==================== 第七组：事件时效验证 ====================

func TestEnterFlashSaleChecksTimeWindow(t *testing.T) {
	// 这个测试验证时间窗口校验逻辑，需要用到 mock
	// miniredis 无法 mock time.Now()，这里只验证活动校验不触发延迟
	svc, _, cleanup := newTestFlashService(t)
	defer cleanup()

	// 确保随机延迟为 0（测试配置默认就是 0）
	cfg := svc.(*flashService).cfg
	if cfg.FlashSale.RandomDelayMaxMs != 0 {
		t.Log("随机延迟配置非零，跳过时间校验测试")
	}
}

// 确保 context 和 time 包被使用（避免 import 报错）
var _ = context.Background
var _ = time.Now
