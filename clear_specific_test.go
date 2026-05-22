package geche

import (
	"context"
	"testing"
	"time"
)

func TestKVCacheClearSpecific(t *testing.T) {
	cache := NewKVCache[string, string]()
	cache.Set("foo", "bar")
	cache.Set("baz", "qux")

	if cache.Len() != 2 {
		t.Fatalf("expected length 2, got %d", cache.Len())
	}

	oldValuesCap := cap(cache.values)
	oldFreelistCap := cap(cache.freelist)
	oldTrieChildrenCap := cap(cache.trie.children)

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("expected length 0, got %d", cache.Len())
	}

	if cap(cache.values) != oldValuesCap {
		t.Errorf("expected values capacity %d, got %d", oldValuesCap, cap(cache.values))
	}
	if cap(cache.freelist) != oldFreelistCap {
		t.Errorf("expected freelist capacity %d, got %d", oldFreelistCap, cap(cache.freelist))
	}
	if cap(cache.trie.children) != oldTrieChildrenCap {
		t.Errorf("expected trie children capacity %d, got %d", oldTrieChildrenCap, cap(cache.trie.children))
	}

	if len(cache.values) != 0 {
		t.Errorf("expected values len 0, got %d", len(cache.values))
	}
	if len(cache.freelist) != 0 {
		t.Errorf("expected freelist len 0, got %d", len(cache.freelist))
	}
	if len(cache.trie.children) != 0 {
		t.Errorf("expected trie children len 0, got %d", len(cache.trie.children))
	}
	if cache.trie.terminal {
		t.Error("expected trie terminal to be false")
	}
	if cache.trie.valueIndex != 0 {
		t.Errorf("expected trie valueIndex 0, got %d", cache.trie.valueIndex)
	}

	// Verify we can insert new values after clear
	cache.Set("a", "b")
	val, err := cache.Get("a")
	if err != nil {
		t.Errorf("unexpected error after clear and set: %v", err)
	}
	if val != "b" {
		t.Errorf("expected b, got %q", val)
	}
}

func TestUpdaterClearSpecific(t *testing.T) {
	updateFn := func(key string) (string, error) {
		return key, nil
	}

	u := NewCacheUpdater(
		NewMapCache[string, string](),
		updateFn,
		2,
	)

	u.Set("key1", "val1")
	u.Set("key2", "val2")

	if u.Len() != 2 {
		t.Fatalf("expected length 2, got %d", u.Len())
	}

	u.Clear()

	if u.Len() != 0 {
		t.Errorf("expected length 0, got %d", u.Len())
	}

	_, err := u.cache.Get("key1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for key1 in underlying cache, got %v", err)
	}

	u.Set("key3", "val3")
	val, err := u.Get("key3")
	if err != nil {
		t.Errorf("unexpected error in Get after post-clear Set: %v", err)
	}
	if val != "val3" {
		t.Errorf("expected val3, got %q", val)
	}
}

func TestTxClearSpecific(t *testing.T) {
	locker := NewLocker(NewMapCache[string, string]())

	// Test writable Tx Clear
	tx := locker.Lock()
	tx.Set("key1", "val1")
	tx.Clear()
	if tx.Len() != 0 {
		t.Errorf("expected length 0, got %d", tx.Len())
	}

	tx.Set("key3", "val3")
	val, err := tx.Get("key3")
	if err != nil {
		t.Errorf("unexpected error in Get after post-clear Set: %v", err)
	}
	if val != "val3" {
		t.Errorf("expected val3, got %q", val)
	}
	tx.Unlock()

	// Test read-only Tx Clear (should panic)
	tx2 := locker.RLock()
	panics := func(f func()) (panicked bool) {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		f()
		return
	}

	if !panics(func() { tx2.Clear() }) {
		t.Error("expected Clear on read-only transaction to panic")
	}
	tx2.Unlock()

	// Test unlocked Tx Clear (should panic)
	if !panics(func() { tx.Clear() }) {
		t.Error("expected Clear on already unlocked transaction to panic")
	}
}

func TestMapTTLCacheClearSpecific(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMapTTLCache[string, string](ctx, time.Minute, time.Minute)
	c.Set("key1", "val1")
	c.Set("key2", "val2")

	if c.head != "key1" || c.tail != "key2" {
		t.Fatalf("unexpected head/tail: %q/%q", c.head, c.tail)
	}

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected length 0, got %d", c.Len())
	}
	if c.head != "" {
		t.Errorf("expected empty head, got %q", c.head)
	}
	if c.tail != "" {
		t.Errorf("expected empty tail, got %q", c.tail)
	}

	c.Set("k3", "v3")
	if c.head != "k3" || c.tail != "k3" {
		t.Errorf("unexpected head/tail after post-clear Set: %q/%q", c.head, c.tail)
	}
	val, err := c.Get("k3")
	if err != nil {
		t.Errorf("unexpected error in Get after post-clear Set: %v", err)
	}
	if val != "v3" {
		t.Errorf("expected v3, got %q", val)
	}
}

func TestRingBufferClearSpecific(t *testing.T) {
	c := NewRingBuffer[string, string](3)
	c.Set("k1", "v1")
	c.Set("k2", "v2")

	if c.Len() != 2 {
		t.Fatalf("expected length 2, got %d", c.Len())
	}

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected length 0, got %d", c.Len())
	}

	if c.head != 0 {
		t.Errorf("expected head 0, got %d", c.head)
	}

	for i, item := range c.data {
		if !item.empty {
			t.Errorf("expected item %d to be empty", i)
		}
		if item.K != "" || item.V != "" {
			t.Errorf("expected item %d to be zeroed out, got key: %q, val: %q", i, item.K, item.V)
		}
	}

	c.Set("k3", "v3")
	val, err := c.Get("k3")
	if err != nil {
		t.Errorf("unexpected error in Get after post-clear Set: %v", err)
	}
	if val != "v3" {
		t.Errorf("expected v3, got %q", val)
	}
}
