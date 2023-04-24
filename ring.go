package geche

import (
	"sync"
)

type bufferRec[T any, K comparable] struct {
	value T
	key K
}

// RingBuffer cache preallocates a fixed number of elements and
// starts overwriting oldest values when this number is reached.
// The idea is to reduce allocations and GC pressure while having
// fixed memory footprint (does not grow).
type RingBuffer[T any, K comparable] struct {
	data []bufferRec[T, K]
	index map[K]int
	head int
	zeroK K
	zeroT T
	mux  sync.RWMutex
}

func NewRingBuffer[T any, K comparable](size int) *RingBuffer[T, K] {
	return &RingBuffer[T, K]{
		data: make([]bufferRec[T, K], size),
		index: make(map[K]int, size),
		zeroK: zero[K](),
		zeroT: zero[T](),
	}
}

// Set adds value to the ring buffer and the index
func (c *RingBuffer[T, K]) Set(key K, value T) {
	c.mux.Lock()
	defer c.mux.Unlock()

	// Remove the key which value we are overwriting
	// from the map. GC does not cleanup preallocated map,
	// so no pressure here.
	if old := c.data[c.head]; old.key != c.zeroK {
		delete(c.index, old.key)
	}

	c.data[c.head] = bufferRec[T, K]{value, key}
	c.index[key] = c.head
	c.head = (c.head + 1) % len(c.data)
}

func (c *RingBuffer[T, K]) Get(key K) (T, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	i, ok := c.index[key]
	if !ok {
		return c.zeroT, ErrNotFound
	}

	return c.data[i].value, nil
}

func (c *RingBuffer[T, K]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.index, key)

	return nil
}
