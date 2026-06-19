// Package semaphore 数据库并发查询信号量 — 限制同时查询 DB 的协程数量
//
// 使用场景：
//   缓存雪崩时大量请求绕过缓存直击数据库，连接池瞬间耗尽。
//   信号量作为第二道防线，限制"同时执行 DB 查询"的协程数量。
//
// 工作原理：
//   使用带缓冲 channel 作为信号量。每个 DB 查询前获取一个槽位（channel <- struct{}{}），
//   查询完成后释放槽位（<-channel）。当槽位满时，新请求阻塞等待。
//
// 面试亮点：channel 的另类用法 — 用 Go channel 实现信号量模式的并发控制
package semaphore

import (
	"context"
	"time"
)

// Limiter 数据库并发查询信号量
type Limiter struct {
	sem     chan struct{}   // 信号量 channel
	timeout time.Duration   // 获取槽位的最大等待时间
}

// New 创建信号量限流器
//
// 参数：
//
//	maxConcurrent — 最大并发数（建议设为 DB 连接池的 50-70%，例如池 50 → 限流 30）
//	waitTimeout   — 等待槽位的最大时间（超时则拒绝请求）
func New(maxConcurrent int, waitTimeout time.Duration) *Limiter {
	return &Limiter{
		sem:     make(chan struct{}, maxConcurrent),
		timeout: waitTimeout,
	}
}

// Acquire 获取一个数据库查询槽位（阻塞直到获取成功或超时）
//
// 返回 true 表示获取成功（调用方负责在查询完成后调用 Release）
// 返回 false 表示超时（调用方应返回错误，不允许查 DB）
func (l *Limiter) Acquire(ctx context.Context) bool {
	select {
	case l.sem <- struct{}{}:
		return true
	case <-time.After(l.timeout):
		return false
	case <-ctx.Done():
		return false
	}
}

// Release 释放一个数据库查询槽位（必须在 Acquire 成功后调用，否则会 panic）
func (l *Limiter) Release() {
	<-l.sem
}

// Available 返回当前可用槽位数（用于监控）
func (l *Limiter) Available() int {
	return cap(l.sem) - len(l.sem)
}

// InUse 返回当前正在使用的槽位数（用于监控）
func (l *Limiter) InUse() int {
	return len(l.sem)
}

// NewDefault 创建默认配置的数据库查询信号量（供 Wire 依赖注入使用）
// 默认：最大 30 并发 DB 查询，等待超时 2 秒
func NewDefault() *Limiter {
	return New(30, 2*time.Second)
}

// Capacity 返回信号量总容量
func (l *Limiter) Capacity() int {
	return cap(l.sem)
}
