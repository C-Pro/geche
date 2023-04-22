package geche

import (
	"sync"
)

// MapCache is the simplest thread-safe map-based cache implementation.
// Does not have any limits or TTL, can grow indefinetly.
type MapCache[T any, K comparable] struct {
	data map[K]T
	mux  sync.RWMutex
}

func NewMapCache[T any, K comparable]() *MapCache[T, K] {
	return &MapCache[T, K]{
		data: make(map[K]T),
	}
}

func (c *MapCache[T, K]) Set(key K, value T) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.data[key] = value
}

func (c *MapCache[T, K]) Get(key K) (T, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	v, ok := c.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (c *MapCache[T, K]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.data, key)

	return nil
}
