package flashinventory

import (
	"sync"
	"sync/atomic"
)

// Inventory 秒杀本地内存库存管理器
// v2: 消除指针悬挂问题——使用 sync.Map + atomic.Int32 替代 RWMutex + 裸指针
type Inventory struct {
	data sync.Map // map[uint]*inventoryEntry
}

type inventoryEntry struct {
	remaining atomic.Int32 // 本地剩余库存
	soldOut   atomic.Bool  // 是否已售罄
}

// New 创建内存库存管理器实例
func New() *Inventory {
	return &Inventory{}
}

// Init 初始化活动的本地库存计数（预热时调用）
func (inv *Inventory) Init(flashSaleID uint, stock int) {
	entry := &inventoryEntry{}
	entry.remaining.Store(int32(stock))
	entry.soldOut.Store(false)
	inv.data.Store(flashSaleID, entry)
}

// Decrement 扣减本地库存计数（Redis扣减成功后调用）
func (inv *Inventory) Decrement(flashSaleID uint) int32 {
	val, ok := inv.data.Load(flashSaleID)
	if !ok {
		return 0
	}
	entry := val.(*inventoryEntry)
	new := entry.remaining.Add(-1)
	if new <= 0 {
		entry.soldOut.Store(true)
	}
	return new
}

// IsSoldOut 检查活动是否已售罄
func (inv *Inventory) IsSoldOut(flashSaleID uint) bool {
	val, ok := inv.data.Load(flashSaleID)
	if !ok {
		return false // 未初始化的活动，由 Redis 判断
	}
	return val.(*inventoryEntry).soldOut.Load()
}

// ResetSoldOut 重置售罄标记（超时释放或库存回补时调用）
// 直接删除整个 entry，让后续请求走 Redis 判断（避免本地计数器与 Redis 不同步）
func (inv *Inventory) ResetSoldOut(flashSaleID uint) {
	inv.data.Delete(flashSaleID)
}

// Increment 增加本地库存计数（库存回补时调用）
func (inv *Inventory) Increment(flashSaleID uint) {
	val, ok := inv.data.Load(flashSaleID)
	if ok {
		val.(*inventoryEntry).remaining.Add(1)
		val.(*inventoryEntry).soldOut.Store(false)
	}
}

// GetRemaining 获取本地记录的剩余库存
func (inv *Inventory) GetRemaining(flashSaleID uint) int {
	val, ok := inv.data.Load(flashSaleID)
	if !ok {
		return 0
	}
	return int(val.(*inventoryEntry).remaining.Load())
}

// Cleanup 清理已结束活动的内存数据
func (inv *Inventory) Cleanup(flashSaleID uint) {
	inv.data.Delete(flashSaleID)
}
