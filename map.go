package geche

import (
	"sync"
)

// MapCache is the simplest thread-safe map-based cache implementation.
// Does not have any limits or TTL, can grow indefinetly.
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

func (c *MapCache[K, V]) Get(key K) (V, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	v, ok := c.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (c *MapCache[K, V]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.data, key)

	return nil
}
