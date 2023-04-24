package geche

import (
	"sync"
)

type bufferRec[K comparable, T any] struct {
	value T
	key   K
}

// RingBuffer cache preallocates a fixed number of elements and
// starts overwriting oldest values when this number is reached.
// The idea is to reduce allocations and GC pressure while having
// fixed memory footprint (does not grow).
type RingBuffer[K comparable, T any] struct {
	data  []bufferRec[K, T]
	index map[K]int
	head  int
	zeroK K
	zeroT T
	mux   sync.RWMutex
}

func NewRingBuffer[K comparable, T any](size int) *RingBuffer[K, T] {
	return &RingBuffer[K, T]{
		data:  make([]bufferRec[K, T], size),
		index: make(map[K]int, size),
		zeroK: zero[K](),
		zeroT: zero[T](),
	}
}

// Set adds value to the ring buffer and the index
func (c *RingBuffer[K, T]) Set(key K, value T) {
	c.mux.Lock()
	defer c.mux.Unlock()

	// Remove the key which value we are overwriting
	// from the map. GC does not cleanup preallocated map,
	// so no pressure here.
	if old := c.data[c.head]; old.key != c.zeroK {
		delete(c.index, old.key)
	}

	c.data[c.head] = bufferRec[K, T]{value, key}
	c.index[key] = c.head
	c.head = (c.head + 1) % len(c.data)
}

func (c *RingBuffer[K, T]) Get(key K) (T, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	i, ok := c.index[key]
	if !ok {
		return c.zeroT, ErrNotFound
	}

	return c.data[i].value, nil
}

func (c *RingBuffer[K, T]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.index, key)

	return nil
}
