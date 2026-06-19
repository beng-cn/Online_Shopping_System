// Package bloom 布隆过滤器 — 缓存穿透的第一道防线
//
// 核心原理：
//   布隆过滤器是一个概率型数据结构，用于快速判断元素"一定不存在"或"可能存在"。
//   - 返回 false → 元素一定不在集合中 → 跳过缓存和数据库查询 → 直接返回 404
//   - 返回 true  → 元素可能存在（有误判率）→ 继续走缓存→数据库的常规流程
//
// 生产实践：
//   1. 启动时从数据库全量加载所有产品 ID，构建布隆过滤器
//   2. 新增产品时实时添加（Add 方法）
//   3. 定时（如每小时）全量重建，清理已删除产品的 ID
//   4. 误判率设计为 0.1%（万分之一），即 1 万个不存在的 ID 中约 10 个会穿透到数据库
//
// 面试亮点：概率型数据结构的工程应用，用极小的内存代价拦截绝大部分无效请求
package bloom

import (
	"hash"
	"hash/fnv"
	"math"
	"sync"
)

// Filter 布隆过滤器 — 线程安全的本地内存布隆过滤器
type Filter struct {
	mu        sync.RWMutex
	bits      []uint64       // 位数组（每个 uint64 存储 64 位）
	size      uint64         // 位数组总位数
	hashFuncs []hash.Hash64  // 哈希函数列表
	seeds     [][]byte       // 每个哈希函数的种子（Reset 后重新写入以保持独立性）
}

// 预设参数（基于预期 100 万产品、0.1% 误判率计算）
// 位数组大小 m：约 14.4 MB（~1150 万位）
// 哈希函数数量 k：约 7 个
const (
	defaultExpectedItems = 1_000_000 // 预期元素数量：100 万
	defaultFalsePositive = 0.001     // 目标误判率：0.1%
)

// NewFilter 创建布隆过滤器（使用默认参数）
func NewFilter() *Filter {
	return NewFilterWithParams(defaultExpectedItems, defaultFalsePositive)
}

// NewFilterWithParams 根据预期元素数量和目标误判率创建布隆过滤器
//
// 计算公式（布隆过滤器理论）：
//
//	位数组大小 m = -n * ln(p) / (ln(2))^2
//	哈希函数数 k = m/n * ln(2)
//
// 参数：
//
//	n — 预期元素数量
//	p — 目标误判率（如 0.001 表示 0.1%）
func NewFilterWithParams(n uint64, p float64) *Filter {
	m := uint64(math.Ceil(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)))
	k := int(math.Ceil(float64(m) / float64(n) * math.Ln2))
	if k < 1 {
		k = 1
	}

	// 位数组大小向上取整到 uint64 的整数倍
	numUint64 := (m + 63) / 64

	funcs, seeds := createHashFunctions(k)

	return &Filter{
		bits:      make([]uint64, numUint64),
		size:      numUint64 * 64,
		hashFuncs: funcs,
		seeds:     seeds,
	}
}

// Add 向布隆过滤器添加元素
func (f *Filter) Add(id uint) {
	f.addUint64(uint64(id))
}

// addUint64 内部实现
func (f *Filter) addUint64(data uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	buf := make([]byte, 8)
	buf[0] = byte(data)
	buf[1] = byte(data >> 8)
	buf[2] = byte(data >> 16)
	buf[3] = byte(data >> 24)
	buf[4] = byte(data >> 32)
	buf[5] = byte(data >> 40)
	buf[6] = byte(data >> 48)
	buf[7] = byte(data >> 56)

	for i, h := range f.hashFuncs {
		h.Reset()
		h.Write(f.seeds[i]) // 重新注入种子（Reset 会清除之前的状态）
		h.Write(buf)
		pos := h.Sum64() % f.size
		wordIdx := pos / 64
		bitIdx := pos % 64
		f.bits[wordIdx] |= 1 << bitIdx
	}
}

// MightContain 检查元素是否可能存在
//
// 返回 false → 一定不存在（可以直接返回 404）
// 返回 true  → 可能存在（有 p 概率误判，需要继续查缓存/数据库）
func (f *Filter) MightContain(id uint) bool {
	return f.mightContainUint64(uint64(id))
}

func (f *Filter) mightContainUint64(data uint64) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	buf := make([]byte, 8)
	buf[0] = byte(data)
	buf[1] = byte(data >> 8)
	buf[2] = byte(data >> 16)
	buf[3] = byte(data >> 24)
	buf[4] = byte(data >> 32)
	buf[5] = byte(data >> 40)
	buf[6] = byte(data >> 48)
	buf[7] = byte(data >> 56)

	for i, h := range f.hashFuncs {
		h.Reset()
		h.Write(f.seeds[i]) // 重新注入种子
		h.Write(buf)
		pos := h.Sum64() % f.size
		wordIdx := pos / 64
		bitIdx := pos % 64
		if f.bits[wordIdx]&(1<<bitIdx) == 0 {
			return false // 有一位为 0 → 一定不存在
		}
	}
	return true // 所有位都为 1 → 可能存在
}

// Size 返回位数组大小（字节）
func (f *Filter) Size() int {
	return len(f.bits) * 8
}

// createHashFunctions 创建 k 个独立的哈希函数及其种子
//
// 使用双重哈希技术：h(i, data) = FNV1a(seed_i || data)
// 每个函数用不同的种子（黄金比例倍数）来产生独立的位置分布。
// 种子需要在每次 Reset() 后重新写入，以保持各函数的独立性。
func createHashFunctions(k int) ([]hash.Hash64, [][]byte) {
	funcs := make([]hash.Hash64, k)
	seeds := make([][]byte, k)
	for i := 0; i < k; i++ {
		h := fnv.New64a()
		seed := uint64(i) * 0x9e3779b97f4a7c15 // 黄金比例的倍数
		buf := []byte{byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24),
			byte(seed >> 32), byte(seed >> 40), byte(seed >> 48), byte(seed >> 56)}
		seeds[i] = buf
		h.Write(buf) // 初始种子写入（第一次使用前不需要 Reset）
		funcs[i] = h
	}
	return funcs, seeds
}
