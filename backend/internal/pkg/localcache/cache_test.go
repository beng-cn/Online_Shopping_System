package localcache

import (
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New(100, 10*time.Minute)
	defer c.Close()

	c.Set("key1", "value1", 5*time.Second)
	val := c.Get("key1")
	if val != "value1" {
		t.Errorf("期望 value1，得到 %v", val)
	}
}

func TestGetExpired(t *testing.T) {
	c := New(100, 10*time.Minute)
	defer c.Close()

	c.Set("key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	val := c.Get("key1")
	if val != nil {
		t.Errorf("过期条目应返回 nil，得到 %v", val)
	}
}

func TestGetNonExistent(t *testing.T) {
	c := New(100, 10*time.Minute)
	defer c.Close()

	if c.Get("nonexistent") != nil {
		t.Error("不存在的 key 应返回 nil")
	}
}

func TestDelete(t *testing.T) {
	c := New(100, 10*time.Minute)
	defer c.Close()

	c.Set("key1", "value1", 5*time.Second)
	c.Delete("key1")

	if c.Get("key1") != nil {
		t.Error("Delete 后 Get 应返回 nil")
	}
}

func TestLRUEviction(t *testing.T) {
	// 容量 5，插入 5 条 → 访问前 3 条（变热）
	// 插入 2 条新数据触发淘汰，验证冷数据（未访问的后 2 条）被淘汰
	c := New(5, 10*time.Minute)
	defer c.Close()

	keys := []string{"a", "b", "c", "d", "e"}
	for _, k := range keys {
		c.Set(k, k, 10*time.Second)
	}
	time.Sleep(1 * time.Millisecond)

	// 访问前 3 条，更新 lastAccess
	for i := 0; i < 3; i++ {
		c.Get(keys[i])
		time.Sleep(1 * time.Millisecond)
	}

	time.Sleep(1 * time.Millisecond)
	// 插入 2 条 → 每次插入触发淘汰 1 条（5*20%=1）
	c.Set("x", "x", 10*time.Second)
	c.Set("y", "y", 10*time.Second)

	// 热数据 a,b,c 应该还在
	for i := 0; i < 3; i++ {
		if c.Get(keys[i]) == nil {
			t.Errorf("热数据 %q 不应被淘汰", keys[i])
		}
	}

	// 新数据 x,y 应在
	if c.Get("x") == nil || c.Get("y") == nil {
		t.Error("新插入的数据应存在")
	}

	// d 或 e（冷数据）至少淘汰了一个
	dAlive := c.Get("d") != nil
	eAlive := c.Get("e") != nil
	if dAlive && eAlive {
		t.Error("冷数据 d 或 e 应被淘汰了一个")
	}
}

func TestEvictPrefersExpiredFirst(t *testing.T) {
	c := New(10, 10*time.Minute)
	defer c.Close()

	for i := 0; i < 5; i++ {
		c.Set(string(rune('a'+i)), i, 10*time.Second)
	}
	for i := 0; i < 5; i++ {
		c.Set(string(rune('z'-i)), i, 1*time.Millisecond)
	}
	time.Sleep(5 * time.Millisecond)

	c.Set("new1", 1, 10*time.Second)

	for i := 0; i < 5; i++ {
		if c.Get(string(rune('a'+i))) == nil {
			t.Errorf("未过期的 '%c' 不应被优先淘汰", rune('a'+i))
		}
	}
}

func TestSize(t *testing.T) {
	c := New(100, 10*time.Minute)
	defer c.Close()

	c.Set("a", 1, 10*time.Second)
	c.Set("b", 2, 10*time.Second)
	c.Set("c", 3, 10*time.Second)

	if c.Size() != 3 {
		t.Errorf("期望 3 条，实际 %d", c.Size())
	}

	c.Delete("a")
	if c.Size() != 2 {
		t.Errorf("删除后期望 2 条，实际 %d", c.Size())
	}
}
