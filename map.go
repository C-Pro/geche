package geche

import (
	"sync"
)

// MapCache is the simplest thread-safe map-based cache implementation.
// Does not have any limits or TTL, can grow indefinitely.
// Should be used when number of distinct keys in the cache is fixed or grows very slow.
type MapCache[K comparable, V any] struct {
	data map[K]V
	mux  sync.RWMutex
}

func NewMapCache[K comparable, V any]() *MapCache[K, V] {
	return &MapCache[K, V]{
		data: make(map[K]V),
	}
}

func (c *MapCache[K, V]) Set(key K, value V) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.data[key] = value
}

func (c *MapCache[K, V]) SetIfPresent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	old, ok := c.data[key]
	if ok {
		c.data[key] = value
		return old, true
	}

	return old, false
}

func (c *MapCache[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	old, ok := c.data[key]
	if ok {
		return old, false
	}

	c.data[key] = value
	return old, true
}

// Get returns ErrNotFound if key does not exist in the cache.
func (c *MapCache[K, V]) Get(key K) (V, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	v, ok := c.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

// Del removes key from the cache. Return value is always nil.
func (c *MapCache[K, V]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.data, key)

	return nil
}

// Snapshot returns a shallow copy of the cache data.
// Locks the cache from modification for the duration of the copy.
func (c *MapCache[K, V]) Snapshot() map[K]V {
	c.mux.RLock()
	defer c.mux.RUnlock()

	snapshot := make(map[K]V, len(c.data))
	for k, v := range c.data {
		snapshot[k] = v
	}

	return snapshot
}

// Len returns number of items in the cache.
func (c *MapCache[K, V]) Len() int {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return len(c.data)
}
