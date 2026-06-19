package breaker

import (
	"testing"
	"time"
)

func TestInitialStateIsClosed(t *testing.T) {
	cb := New(5, 30*time.Second)
	if cb.State() != StateClosed {
		t.Error("新创建的熔断器状态应为 CLOSED")
	}
}

func TestClosedAllowsRequests(t *testing.T) {
	cb := New(5, 30*time.Second)
	if !cb.Allow() {
		t.Error("CLOSED 状态应允许请求通过")
	}
}

func TestOpenAfterMaxErrors(t *testing.T) {
	cb := New(3, 30*time.Second)

	// 前 3 次 Allow 都通过（CLOSED 状态允许多次 Allow）
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("第 %d 次 Allow 应通过（CLOSED 状态）", i+1)
		}
	}

	// 记录 3 次失败，应触发熔断
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Error("连续 3 次失败后应进入 OPEN 状态")
	}

	// OPEN 状态下拒绝请求
	if cb.Allow() {
		t.Error("OPEN 状态应拒绝请求")
	}
}

func TestHalfOpenAfterTimeout(t *testing.T) {
	cb := New(2, 10*time.Millisecond)

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Fatal("熔断未触发")
	}

	// 等待超时
	time.Sleep(15 * time.Millisecond)

	// 超时后第一个请求应进入 HALF-OPEN 并允许通过
	if !cb.Allow() {
		t.Error("超时后第一个请求应允许通过（HALF-OPEN 试探）")
	}

	// 第二个请求在半开状态应被拒绝
	if cb.Allow() {
		t.Error("HALF-OPEN 状态只允许一个试探请求")
	}
}

func TestHalfOpenSuccessRecovers(t *testing.T) {
	cb := New(2, 10*time.Millisecond)

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}
	time.Sleep(15 * time.Millisecond)

	// 试探请求通过
	cb.Allow()
	// 记录成功 → 恢复 CLOSED
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Error("半开试探成功后应恢复 CLOSED")
	}

	// 后续请求正常通过
	if !cb.Allow() {
		t.Error("恢复 CLOSED 后请求应正常通过")
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	cb := New(2, 20*time.Millisecond)

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}
	time.Sleep(25 * time.Millisecond)

	// 试探请求
	cb.Allow()
	// 试探失败 → 重新熔断
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Error("半开试探失败后应重新进入 OPEN")
	}
}

func TestReset(t *testing.T) {
	cb := New(2, 30*time.Second)

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Fatal("熔断未触发")
	}

	// 手动重置
	cb.Reset()

	if cb.State() != StateClosed {
		t.Error("Reset() 后应恢复 CLOSED")
	}
	if !cb.Allow() {
		t.Error("Reset() 后请求应正常通过")
	}
}
