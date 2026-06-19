package bloom

import (
	"testing"
)

func TestAddAndMightContain(t *testing.T) {
	f := NewFilterWithParams(1000, 0.01)

	f.Add(42)
	if !f.MightContain(42) {
		t.Error("Add(42) 后 MightContain(42) 应返回 true")
	}
}

func TestNotContain(t *testing.T) {
	f := NewFilterWithParams(1000, 0.01)

	if f.MightContain(1) {
		t.Error("未添加的 ID 不应返回 true")
	}
}

func TestMultipleAdd(t *testing.T) {
	f := NewFilterWithParams(10000, 0.001)

	ids := []uint{1, 10, 100, 1000, 10000, 99999}
	for _, id := range ids {
		f.Add(id)
	}

	for _, id := range ids {
		if !f.MightContain(id) {
			t.Errorf("Add(%d) 后 MightContain(%d) 应返回 true", id, id)
		}
	}
}

func TestFalsePositiveRate(t *testing.T) {
	n := uint64(10000)
	p := 0.001
	f := NewFilterWithParams(n, p)

	for i := uint(0); i < uint(n); i++ {
		f.Add(i)
	}

	// 测试 100000 个不存在的 ID
	testCount := 100000
	falsePositives := 0
	for i := uint(n); i < uint(n)+uint(testCount); i++ {
		if f.MightContain(i) {
			falsePositives++
		}
	}

	actualRate := float64(falsePositives) / float64(testCount)
	t.Logf("误判率: 实际=%.4f 目标=%.4f (%d/%d)", actualRate, p, falsePositives, testCount)

	// 实际误判率应显著低于修复前的 6.6%，在 1% 以内即可验证种子生效
	if actualRate > 0.01 {
		t.Errorf("误判率 %.4f 过高(>1%%)", actualRate)
	}
}

func TestAllHashFunctionsProduceDiff(t *testing.T) {
	// 验证 7 个哈希函数对同一输入产生不同位置（修复种子 bug 后）
	f := NewFilterWithParams(10000, 0.001)
	data := uint64(42)
	buf := make([]byte, 8)
	for j := 0; j < 8; j++ {
		buf[j] = byte(data >> (j * 8))
	}

	positions := make(map[uint64]bool)
	for i, h := range f.hashFuncs {
		h.Reset()
		h.Write(f.seeds[i])
		h.Write(buf)
		pos := h.Sum64() % f.size
		positions[pos] = true
	}

	// 7 个函数应产生至少 5 个不同位置（允许极小概率碰撞）
	if len(positions) < 5 {
		t.Errorf("7 个哈希函数仅产生 %d 个不同位置，种子可能未生效", len(positions))
	}
	t.Logf("7 个哈希函数产生 %d 个不同位置", len(positions))
}

func TestSize(t *testing.T) {
	n := uint64(1000000)
	p := 0.001
	f := NewFilterWithParams(n, p)

	size := f.Size()
	t.Logf("100万元素/0.1%%误判率 → 内存: %d bytes (%.1f MB)", size, float64(size)/1024/1024)

	if size <= 0 {
		t.Error("Size() 应返回正数")
	}
}
