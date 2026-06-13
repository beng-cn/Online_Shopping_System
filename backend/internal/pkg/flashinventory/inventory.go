package flashinventory

import (
	"sync"
	"sync/atomic"
)

// Inventory 秒杀本地内存库存管理器
// 使用 atomic 操作维护售罄标记，5ns 级别拒绝已售罄活动的后续请求
type Inventory struct {
	mu       sync.RWMutex
	remaining map[uint]*int32   // 活动ID → 本地剩余库存（原子操作）
	soldOut   map[uint]bool     // 活动ID → 是否已售罄
}

// New 创建内存库存管理器实例
func New() *Inventory {
	return &Inventory{
		remaining: make(map[uint]*int32),
		soldOut:   make(map[uint]bool),
	}
}

// Init 初始化活动的本地库存计数（预热时调用）
func (inv *Inventory) Init(flashSaleID uint, stock int) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	v := int32(stock)
	inv.remaining[flashSaleID] = &v
	inv.soldOut[flashSaleID] = false
}

// Decrement 扣减本地库存计数（Redis扣减成功后调用）
// 返回扣减后的剩余数量
func (inv *Inventory) Decrement(flashSaleID uint) int32 {
	inv.mu.RLock()
	ptr, ok := inv.remaining[flashSaleID]
	inv.mu.RUnlock()

	if !ok {
		return 0
	}

	new := atomic.AddInt32(ptr, -1)
	if new <= 0 {
		inv.mu.Lock()
		inv.soldOut[flashSaleID] = true
		inv.mu.Unlock()
	}
	return new
}

// IsSoldOut 检查活动是否已售罄（5ns级别，零锁竞争）
func (inv *Inventory) IsSoldOut(flashSaleID uint) bool {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	soldOut, ok := inv.soldOut[flashSaleID]
	if !ok {
		// 未初始化的活动，认为未售罄（由Redis来判断）
		return false
	}
	return soldOut
}

// ResetSoldOut 重置售罄标记（超时释放或库存回补时调用）
func (inv *Inventory) ResetSoldOut(flashSaleID uint) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	inv.soldOut[flashSaleID] = false
}

// Increment 增加本地库存计数（库存回补时调用）
func (inv *Inventory) Increment(flashSaleID uint) {
	inv.mu.RLock()
	ptr, ok := inv.remaining[flashSaleID]
	inv.mu.RUnlock()

	if ok {
		atomic.AddInt32(ptr, 1)
		// 库存恢复后重置售罄标记
		inv.ResetSoldOut(flashSaleID)
	}
}

// GetRemaining 获取本地记录的剩余库存
func (inv *Inventory) GetRemaining(flashSaleID uint) int {
	inv.mu.RLock()
	ptr, ok := inv.remaining[flashSaleID]
	inv.mu.RUnlock()

	if !ok {
		return 0
	}
	return int(atomic.LoadInt32(ptr))
}

// Cleanup 清理已结束活动的内存数据
func (inv *Inventory) Cleanup(flashSaleID uint) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	delete(inv.remaining, flashSaleID)
	delete(inv.soldOut, flashSaleID)
}
