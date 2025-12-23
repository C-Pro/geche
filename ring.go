package geche

import (
	"iter"
	"sync"
)

type BufferRec[K comparable, V any] struct {
	K     K
	V     V
	empty bool
}

// RingBuffer cache preallocates a fixed number of elements and
// starts overwriting oldest values when this number is reached.
// The idea is to reduce allocations and GC pressure while having
// fixed memory footprint (does not grow).
type RingBuffer[K comparable, V any] struct {
	data  []BufferRec[K, V]
	index map[K]int
	head  int
	zeroV V
	mux   sync.RWMutex
}

// NewRingBuffer creates RingBuffer instance with predifined size (number of records).
// This number of records is preallocated immediately. RingBuffer cache can't hold more
// than size values.
func NewRingBuffer[K comparable, V any](size int) *RingBuffer[K, V] {
	b := RingBuffer[K, V]{
		data:  make([]BufferRec[K, V], size),
		index: make(map[K]int, size),
		zeroV: zero[V](),
	}

	for i := 0; i < size; i++ {
		b.data[i].empty = true
	}

	return &b
}

// Set adds value to the ring buffer and key index.
func (c *RingBuffer[K, V]) Set(key K, value V) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.set(key, value)
}

func (c *RingBuffer[K, V]) set(key K, value V) {
	// Remove the key which value we are overwriting
	// from the map. GC does not cleanup preallocated map,
	// so no pressure here.
	if old := c.data[c.head]; !old.empty {
		delete(c.index, old.K)
	}

	c.data[c.head].K = key
	c.data[c.head].V = value
	c.data[c.head].empty = false
	c.index[key] = c.head
	c.head = (c.head + 1) % len(c.data)
}

func (c *RingBuffer[K, V]) SetIfPresent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	i, present := c.index[key]
	if present {
		oldVal := c.data[i].V
		c.data[i].V = value
		return oldVal, present
	}

	return c.zeroV, false
}

func (c *RingBuffer[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	i, present := c.index[key]
	if present {
		return c.data[i].V, false
	}

	c.set(key, value)
	return c.zeroV, true
}

// Get returns cached value for the key, or ErrNotFound if the key does not exist.
func (c *RingBuffer[K, V]) Get(key K) (V, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	i, ok := c.index[key]
	if !ok {
		return c.zeroV, ErrNotFound
	}

	return c.data[i].V, nil
}

// Del removes key from the cache. Return value is always nil.
func (c *RingBuffer[K, V]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	idx, ok := c.index[key]
	if !ok {
		return nil
	}

	// Mark item as deleted.
	c.data[idx].empty = true
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
		snapshot[k] = c.data[i].V
	}

	return snapshot
}

// Len returns number of items in the cache.
func (c *RingBuffer[K, V]) Len() int {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return len(c.index)
}

// ListAll returns all key-value pairs in the cache in the order they were added.
func (c *RingBuffer[K, V]) ListAll() []BufferRec[K, V] {
	c.mux.RLock()
	defer c.mux.RUnlock()

	res := make([]BufferRec[K, V], 0, len(c.index))
	for i := 0; i < len(c.data); i++ {
		idx := (c.head + i) % len(c.data)
		if c.data[idx].empty {
			continue // Skip empty items.
		}
		res = append(res, BufferRec[K, V]{K: c.data[idx].K, V: c.data[idx].V})
	}

	return res
}

// ListAllValues returns all values in the cache in the order they were added.
func (c *RingBuffer[K, V]) ListAllValues() []V {
	c.mux.RLock()
	defer c.mux.RUnlock()

	res := make([]V, 0, len(c.index))
	for i := 0; i < len(c.data); i++ {
		idx := (c.head + i) % len(c.data)
		if c.data[idx].empty {
			continue
		}
		res = append(res, c.data[idx].V)
	}
	return res
}

// ListAllKeys returns all keys in the cache in the order they were added.
func (c *RingBuffer[K, V]) ListAllKeys() []K {
	c.mux.RLock()
	defer c.mux.RUnlock()

	res := make([]K, 0, len(c.index))
	for i := 0; i < len(c.data); i++ {
		idx := (c.head + i) % len(c.data)
		if c.data[idx].empty {
			continue
		}
		res = append(res, c.data[idx].K)
	}
	return res
}

// All is a (read-only) iterator over all key-value pairs in the cache.
// Attempt to modify the cache (Set/Del, etc.) while iterating will lead to
// a deadlock.
func (c *RingBuffer[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		c.mux.RLock()
		defer c.mux.RUnlock()

		for i := 0; i < len(c.data); i++ {
			idx := (c.head + i) % len(c.data)
			if c.data[idx].empty {
				continue
			}
			if !yield(c.data[idx].K, c.data[idx].V) {
				break
			}
		}
	}
}
