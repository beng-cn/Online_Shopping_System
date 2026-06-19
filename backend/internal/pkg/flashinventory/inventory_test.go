package flashinventory

import (
	"testing"
)

func TestInitAndIsSoldOut(t *testing.T) {
	inv := New()
	inv.Init(1, 100)

	if inv.IsSoldOut(1) {
		t.Error("库存 100 时不应售罄")
	}
}

func TestIsSoldOutAfterDecrementToZero(t *testing.T) {
	inv := New()
	inv.Init(1, 3)

	inv.Decrement(1)
	inv.Decrement(1)
	inv.Decrement(1)

	if !inv.IsSoldOut(1) {
		t.Error("库存扣到 0 后应标记售罄")
	}
}

func TestIncrementRemovesSoldOut(t *testing.T) {
	inv := New()
	inv.Init(1, 1)
	inv.Decrement(1)

	if !inv.IsSoldOut(1) {
		t.Fatal("先确认售罄")
	}

	inv.Increment(1)

	if inv.IsSoldOut(1) {
		t.Error("库存回补后售罄标记应清除")
	}
}

func TestUnknownActivity(t *testing.T) {
	inv := New()

	if inv.IsSoldOut(999) {
		t.Error("未初始化的活动 IsSoldOut 应返回 false")
	}

	if inv.Decrement(999) != 0 {
		t.Error("Decrement 未初始化活动应返回 0")
	}
}

func TestResetSoldOut(t *testing.T) {
	inv := New()
	inv.Init(1, 1)
	// 扣到 0 触发售罄
	inv.Decrement(1)

	if !inv.IsSoldOut(1) {
		t.Fatal("扣到 0 后应售罄")
	}

	inv.ResetSoldOut(1)

	if inv.IsSoldOut(1) {
		t.Error("ResetSoldOut 后不应再标记售罄")
	}
}

func TestCleanup(t *testing.T) {
	inv := New()
	inv.Init(1, 100)
	inv.Cleanup(1)

	if inv.GetRemaining(1) != 0 {
		t.Error("Cleanup 后库存应为 0")
	}
}

func TestGetRemaining(t *testing.T) {
	inv := New()
	inv.Init(1, 50)

	if rem := inv.GetRemaining(1); rem != 50 {
		t.Errorf("期望剩余 50，实际 %d", rem)
	}

	inv.Decrement(1)
	if rem := inv.GetRemaining(1); rem != 49 {
		t.Errorf("扣减后期望 49，实际 %d", rem)
	}
}
