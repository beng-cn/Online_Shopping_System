package flashlimiter

import (
	"sync"
	"testing"
	"time"
)

func TestAllowFirstRequest(t *testing.T) {
	l := NewUserRateLimiter(1, 1)
	if !l.Allow(1) {
		t.Error("首次请求应允许")
	}
}

func TestDenyWhenOutOfTokens(t *testing.T) {
	l := NewUserRateLimiter(1, 1)

	l.Allow(1)
	if l.Allow(1) {
		t.Error("令牌耗尽后应拒绝")
	}
}

func TestTokenRecovery(t *testing.T) {
	l := NewUserRateLimiter(2, 2)

	l.Allow(1)
	l.Allow(1)

	if l.Allow(1) {
		t.Error("令牌耗尽后应立即拒绝")
	}

	time.Sleep(1100 * time.Millisecond)

	if !l.Allow(1) {
		t.Error("1 秒后令牌应已恢复")
	}
	if !l.Allow(1) {
		t.Error("应恢复 2 个令牌")
	}
}

func TestPerUserIndependent(t *testing.T) {
	l := NewUserRateLimiter(1, 1)

	if !l.Allow(1) {
		t.Fatal("用户 1 第一次应允许")
	}

	if !l.Allow(2) {
		t.Error("不同用户的限流应独立")
	}

	if l.Allow(1) {
		t.Error("用户 1 令牌耗尽")
	}
}

func TestBurst(t *testing.T) {
	l := NewUserRateLimiter(1, 5)

	for i := 0; i < 5; i++ {
		if !l.Allow(1) {
			t.Fatalf("burst 内第 %d 次应允许", i+1)
		}
	}

	if l.Allow(1) {
		t.Error("burst 耗尽后应拒绝")
	}
}

func TestCleanupExpired(t *testing.T) {
	l := NewUserRateLimiter(1, 1)

	l.Allow(1)

	// 等一小段时间确保 lastTime 已过
	time.Sleep(10 * time.Millisecond)
	l.CleanupExpired(1 * time.Millisecond)

	// 清理后桶被删除，再访问应重建
	if !l.Allow(1) {
		t.Error("清理后重新创建桶，应有令牌")
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := NewUserRateLimiter(100, 100)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if l.Allow(uint(id % 10)) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	if successCount != 100 {
		t.Errorf("期望 100 次通过，实际 %d", successCount)
	}
}
