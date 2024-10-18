package geche

import (
	"sync"
)

type bufferRec[K comparable, V any] struct {
	value V
	key   K
}

// RingBuffer cache preallocates a fixed number of elements and
// starts overwriting oldest values when this number is reached.
// The idea is to reduce allocations and GC pressure while having
// fixed memory footprint (does not grow).
type RingBuffer[K comparable, V any] struct {
	data  []bufferRec[K, V]
	index map[K]int
	head  int
	zeroK K
	zeroV V
	mux   sync.RWMutex
}

// NewRingBuffer creates RingBuffer instance with predifined size (number of records).
// This number of records is preallocated immediately. RingBuffer cache can't hold more
// than size values.
func NewRingBuffer[K comparable, V any](size int) *RingBuffer[K, V] {
	return &RingBuffer[K, V]{
		data:  make([]bufferRec[K, V], size),
		index: make(map[K]int, size),
		zeroK: zero[K](),
		zeroV: zero[V](),
	}
}

// Set adds value to the ring buffer and key index.
func (c *RingBuffer[K, V]) Set(key K, value V) {
	c.mux.Lock()
	defer c.mux.Unlock()
	// Remove the key which value we are overwriting
	// from the map. GC does not cleanup preallocated map,
	// so no pressure here.
	if old := c.data[c.head]; old.key != c.zeroK {
		delete(c.index, old.key)
	}

	c.data[c.head].key = key
	c.data[c.head].value = value
	c.index[key] = c.head
	c.head = (c.head + 1) % len(c.data)
}

func (c *RingBuffer[K, V]) SetIfPresent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	i, present := c.index[key]
	if present {
		oldVal := c.data[i].value
		c.data[i].value = value
		return oldVal, present
	}

	return c.zeroV, false
}

// Get returns cached value for the key, or ErrNotFound if the key does not exist.
func (c *RingBuffer[K, V]) Get(key K) (V, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	i, ok := c.index[key]
	if !ok {
		return c.zeroV, ErrNotFound
	}

	return c.data[i].value, nil
}

// Del removes key from the cache. Return value is always nil.
func (c *RingBuffer[K, V]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.index, key)

	return nil
}

// Snapshot returns a shallow copy of the cache data.
// Locks the cache from modification for the duration of the copy.
func (c *RingBuffer[K, V]) Snapshot() map[K]V {
	c.mux.RLock()
	defer c.mux.RUnlock()

	snapshot := make(map[K]V, len(c.index))
	for k, i := range c.index {
		snapshot[k] = c.data[i].value
	}

	return snapshot
}

// Len returns number of items in the cache.
func (c *RingBuffer[K, V]) Len() int {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return len(c.index)
}
