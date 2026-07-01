package redis

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

// newTestCache 创建测试用的 FlashCache 实例
func newTestCache(t *testing.T) (FlashCache, *miniredis.Miniredis, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("启动 miniredis 失败: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	cache := NewFlashCache(client)

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return cache, mr, cleanup
}

// ==================== A组：AtomicSetStock Lua 脚本修复验证 ====================

// TestAtomicSetStock_KeyNotExists_CreatesKey (A1)
// 验证：当库存 Key 不存在时，CAS 脚本直接创建 Key 而非返回失败
func TestAtomicSetStock_KeyNotExists_CreatesKey(t *testing.T) {
	cache, _, cleanup := newTestCache(t)
	defer cleanup()

	flashID := uint(99901)

	// 确保 Key 不存在（miniredis 是干净的，无需额外操作）

	// 调用 CAS：expected=0（读取到的值）, newVal=100（修正值）
	code, val, err := cache.AtomicSetStock(flashID, 0, 100)
	if err != nil {
		t.Fatalf("AtomicSetStock 失败: %v", err)
	}
	if code != 1 {
		t.Errorf("A1 失败: 期望 code=1（CAS 成功创建Key），实际 code=%d", code)
	}
	if val != 100 {
		t.Errorf("A1 失败: 期望 val=100，实际 val=%d", val)
	}

	// 验证 Key 确实被创建
	exists, err := cache.StockKeyExists(flashID)
	if err != nil {
		t.Fatalf("StockKeyExists 失败: %v", err)
	}
	if !exists {
		t.Error("A1 失败: 期望 Key 被创建，但 StockKeyExists 返回 false")
	}
}

// TestAtomicSetStock_KeyExists_CasSuccess (A2)
// 验证：Key 存在且 expected 匹配时 CAS 成功
func TestAtomicSetStock_KeyExists_CasSuccess(t *testing.T) {
	cache, _, cleanup := newTestCache(t)
	defer cleanup()

	flashID := uint(99902)

	// 预热 Key = 50
	if err := cache.WarmUpStock(flashID, 50); err != nil {
		t.Fatalf("预热失败: %v", err)
	}

	// CAS: expected=50, newVal=80（修正库存）
	code, val, err := cache.AtomicSetStock(flashID, 50, 80)
	if err != nil {
		t.Fatalf("AtomicSetStock 失败: %v", err)
	}
	if code != 1 {
		t.Errorf("A2 失败: 期望 code=1（CAS 成功），实际 code=%d", code)
	}
	if val != 80 {
		t.Errorf("A2 失败: 期望 val=80，实际 val=%d", val)
	}
}

// TestAtomicSetStock_KeyExists_CasConflict (A3)
// 验证：Key 存在但 expected 不匹配时 CAS 放弃修正
func TestAtomicSetStock_KeyExists_CasConflict(t *testing.T) {
	cache, _, cleanup := newTestCache(t)
	defer cleanup()

	flashID := uint(99903)

	// 预热 Key = 50
	if err := cache.WarmUpStock(flashID, 50); err != nil {
		t.Fatalf("预热失败: %v", err)
	}

	// CAS: expected=30（与实际的 50 不匹配），newVal=80
	code, val, err := cache.AtomicSetStock(flashID, 30, 80)
	if err != nil {
		t.Fatalf("AtomicSetStock 失败: %v", err)
	}
	if code != 0 {
		t.Errorf("A3 失败: 期望 code=0（CAS 冲突放弃），实际 code=%d", code)
	}
	if val != 50 {
		t.Errorf("A3 失败: 期望 val=50（原值不变），实际 val=%d", val)
	}

	// 验证 Key 未被修改
	remaining, err := cache.GetRemainingStock(flashID)
	if err != nil {
		t.Fatalf("GetRemainingStock 失败: %v", err)
	}
	if remaining != 50 {
		t.Errorf("A3 失败: 期望库存保持50不变，实际=%d", remaining)
	}
}

// ==================== C组：StockKeyExists 新方法验证 ====================

// TestStockKeyExists_True (C1)
// 验证：Key 存在时返回 true
func TestStockKeyExists_True(t *testing.T) {
	cache, _, cleanup := newTestCache(t)
	defer cleanup()

	flashID := uint(99904)

	if err := cache.WarmUpStock(flashID, 10); err != nil {
		t.Fatalf("预热失败: %v", err)
	}

	exists, err := cache.StockKeyExists(flashID)
	if err != nil {
		t.Fatalf("StockKeyExists 失败: %v", err)
	}
	if !exists {
		t.Error("C1 失败: 期望 StockKeyExists 返回 true")
	}
}

// TestStockKeyExists_False (C2)
// 验证：Key 不存在时返回 false
func TestStockKeyExists_False(t *testing.T) {
	cache, _, cleanup := newTestCache(t)
	defer cleanup()

	flashID := uint(99905)

	// 确保 Key 不存在

	exists, err := cache.StockKeyExists(flashID)
	if err != nil {
		t.Fatalf("StockKeyExists 失败: %v", err)
	}
	if exists {
		t.Error("C2 失败: 期望 StockKeyExists 返回 false")
	}
}

// TestStockKeyExists_DistinguishesZeroStock (边界测试)
// 验证：StockKeyExists 能区分"库存为0（Key 存在）"和"Key 不存在"
func TestStockKeyExists_DistinguishesZeroStock(t *testing.T) {
	cache, _, cleanup := newTestCache(t)
	defer cleanup()

	flashID := uint(99906)

	// 场景1：Key 不存在 → false
	exists, err := cache.StockKeyExists(flashID)
	if err != nil {
		t.Fatalf("StockKeyExists 失败: %v", err)
	}
	if exists {
		t.Error("边界测试失败: Key 不存在时期望 false")
	}

	// 场景2：预热 Key=0（模拟库存售罄）
	if err := cache.WarmUpStock(flashID, 0); err != nil {
		t.Fatalf("预热失败: %v", err)
	}

	// GetRemainingStock 返回 0（无法区分）
	remaining, err := cache.GetRemainingStock(flashID)
	if err != nil {
		t.Fatalf("GetRemainingStock 失败: %v", err)
	}
	if remaining != 0 {
		t.Errorf("期望 remaining=0，实际=%d", remaining)
	}

	// StockKeyExists 返回 true（正确区分！）
	exists, err = cache.StockKeyExists(flashID)
	if err != nil {
		t.Fatalf("StockKeyExists 失败: %v", err)
	}
	if !exists {
		t.Error("边界测试失败: 库存为0但Key存在时期望 true")
	}
}
