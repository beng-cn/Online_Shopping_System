// Package breaker 数据库断路器 — 缓存雪崩时保护数据库的最后防线
//
// 核心状态机：
//
//	         ┌──────────────────────┐
//	         │     CLOSED（闭合）     │ ← 正常状态，请求通过
//	         │   errorCount < maxErr  │
//	         └──────┬───────────────┘
//	                │ errorCount >= maxErr
//	                ▼
//	         ┌──────────────────────┐
//	         │     OPEN（断开）       │ ← 熔断状态，请求直接拒绝
//	         │   等待 timeout 超时     │
//	         └──────┬───────────────┘
//	                │ timeout 到期
//	                ▼
//	         ┌──────────────────────┐
//	         │  HALF-OPEN（半开）    │ ← 试探状态，允许少量请求通过
//	         │  一个请求成功 → CLOSED  │
//	         │  一个请求失败 → OPEN    │
//	         └──────────────────────┘
//
// 面试亮点：熔断器模式的 Go 实现，理解微服务弹性设计的三个状态转换
package breaker

import (
	"sync"
	"sync/atomic"
	"time"
)

// State 熔断器状态
type State int32

const (
	StateClosed    State = iota // 闭合（正常）
	StateOpen                   // 断开（熔断）
	StateHalfOpen               // 半开（试探）
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker 数据库熔断器
type CircuitBreaker struct {
	mu sync.Mutex

	maxErrors   int           // 最大连续错误数（触发熔断阈值）
	timeout     time.Duration // 熔断持续时间（超时后进入半开状态）
	windowSize  time.Duration // 滑动窗口大小

	state       State
	errorCount  int32         // 连续错误计数（原子操作）
	lastFailure time.Time     // 最后一次失败时间
	openedAt    time.Time     // 熔断触发时间
}

// New 创建熔断器
//
// 参数：
//
//	maxErrors  — 触发熔断的连续错误数阈值（如 5）
//	timeout    — 熔断持续时间（如 30 秒）
func New(maxErrors int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxErrors: maxErrors,
		timeout:   timeout,
		state:     StateClosed,
	}
}

// Allow 检查是否允许请求通过
//
// 返回 true 表示允许通过（当前状态为 CLOSED 或 HALF-OPEN）
// 返回 false 表示拒绝请求（当前状态为 OPEN）
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查熔断是否已超时
		if time.Since(cb.openedAt) >= cb.timeout {
			cb.state = StateHalfOpen
			return true // 半开状态允许一个试探请求通过
		}
		return false
	case StateHalfOpen:
		return false // 半开状态只允许一个请求通过（已被第一个 Allow 消耗）
	default:
		return true
	}
}

// RecordSuccess 记录成功（在半开状态成功后恢复闭合）
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt32(&cb.errorCount, 0)
	cb.state = StateClosed
}

// RecordFailure 记录失败（连续失败达到阈值时触发熔断）
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	count := atomic.AddInt32(&cb.errorCount, 1)
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen || (cb.state == StateClosed && int(count) >= cb.maxErrors) {
		cb.state = StateOpen
		cb.openedAt = time.Now()
	}
}

// State 返回当前熔断器状态
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// NewDefault 创建默认配置的熔断器（供 Wire 依赖注入使用）
// 默认：连续 5 次错误触发熔断，30 秒后进入半开试探
func NewDefault() *CircuitBreaker {
	return New(5, 30*time.Second)
}

// Reset 手动重置熔断器（运维操作）
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt32(&cb.errorCount, 0)
	cb.state = StateClosed
}
