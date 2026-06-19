package semaphore

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	l := New(3, 1*time.Second)
	ctx := context.Background()

	// 可以获取 3 个槽位
	if !l.Acquire(ctx) {
		t.Error("第 1 个槽位应成功获取")
	}
	if !l.Acquire(ctx) {
		t.Error("第 2 个槽位应成功获取")
	}
	if !l.Acquire(ctx) {
		t.Error("第 3 个槽位应成功获取")
	}

	// 第 4 个应该超时
	if l.Acquire(ctx) {
		t.Error("槽位已满，第 4 个应超时拒绝")
	}

	// 释放一个槽位
	l.Release()

	// 现在可以获取
	if !l.Acquire(ctx) {
		t.Error("释放槽位后应可重新获取")
	}
}

func TestAcquireTimeout(t *testing.T) {
	l := New(1, 10*time.Millisecond)
	ctx := context.Background()

	// 占满槽位
	l.Acquire(ctx)

	start := time.Now()
	ok := l.Acquire(ctx)
	elapsed := time.Since(start)

	if ok {
		t.Error("槽位已满时应超时返回 false")
	}
	if elapsed < 5*time.Millisecond {
		t.Errorf("应等待足够时间再超时，实际 %v", elapsed)
	}
}

func TestConcurrentAcquire(t *testing.T) {
	l := New(2, 2*time.Second)
	ctx := context.Background()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Acquire(ctx) {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				l.Release()
			}
		}()
	}

	wg.Wait()
	// 信号量容量为 2，共 10 个协程并发，所有都应最终获取到（因为有 Release）
	if successCount != 10 {
		t.Errorf("10 个协程应全部获取成功，实际 %d", successCount)
	}
}

func TestAvailable(t *testing.T) {
	l := New(5, 1*time.Second)
	ctx := context.Background()

	if l.Available() != 5 {
		t.Errorf("初始可用槽位应为 5，实际 %d", l.Available())
	}

	l.Acquire(ctx)
	l.Acquire(ctx)

	if l.Available() != 3 {
		t.Errorf("获取 2 个后可用槽位应为 3，实际 %d", l.Available())
	}

	l.Release()
	if l.Available() != 4 {
		t.Errorf("释放 1 个后可用槽位应为 4，实际 %d", l.Available())
	}
}
