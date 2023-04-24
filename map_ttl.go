package geche

import (
	"context"
	"sync"
	"time"
)

// defaultCleanupInterval controls how often cache will purge obsolete values.
const defaultCleanupInterval = time.Second

type ttlRec[K comparable, V any] struct {
	// linked list to maintain order
	prev      K
	next      K
	value     V
	timestamp time.Time
}

// zero returns zero value for the type T.
func zero[T any]() T {
	var z T
	return z
}

// MapTTLCache is the thread-safe map-based cache with TTL support.
type MapTTLCache[K comparable, V any] struct {
	data map[K]ttlRec[K, V]
	mux  sync.RWMutex
	ttl  time.Duration
	now  func() time.Time
	tail K
	head K
	zero K
}

func NewMapTTLCache[K comparable, V any](
	ctx context.Context,
	ttl time.Duration,
	cleanupInterval time.Duration,
) *MapTTLCache[K, V] {
	if cleanupInterval == 0 {
		cleanupInterval = defaultCleanupInterval
	}
	c := MapTTLCache[K, V]{
		data: make(map[K]ttlRec[K, V]),
		ttl:  ttl,
		now:  time.Now,
		zero: zero[K](), // cache zero value for comparisons.
	}

	go func(ctx context.Context) {
		t := time.NewTicker(cleanupInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = c.cleanup()
			}
		}
	}(ctx)

	return &c
}

func (c *MapTTLCache[K, V]) Set(key K, value V) {
	c.mux.Lock()
	defer c.mux.Unlock()

	val := ttlRec[K, V]{
		value:     value,
		prev:      c.tail,
		timestamp: c.now(),
	}

	if c.head == c.zero {
		c.head = key
		c.tail = key
		val.prev = c.zero
		c.data[key] = val
		return
	}

	// If the record for this key already exists
	// and is somewhere in the middle of the list
	// removing it before adding to the tail.
	if rec, ok := c.data[key]; ok && key != c.tail {
		prev := c.data[rec.prev]
		next := c.data[rec.next]
		prev.next = rec.next
		next.prev = rec.prev
		c.data[rec.prev] = prev
		c.data[rec.next] = next
	}

	tailval := c.data[c.tail]
	tailval.next = key
	c.data[c.tail] = tailval
	c.tail = key
	c.data[key] = val
}

func (c *MapTTLCache[K, V]) Get(key K) (V, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	v, ok := c.data[key]
	if !ok {
		return v.value, ErrNotFound
	}

	if c.now().Sub(v.timestamp) >= c.ttl {
		return v.value, ErrNotFound
	}

	return v.value, nil
}

func (c *MapTTLCache[K, V]) Del(key K) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	rec, ok := c.data[key]
	if !ok {
		return nil
	}

	delete(c.data, key)

	if key == c.head {
		c.head = rec.next
	}

	if key == c.tail {
		c.tail = rec.prev
	}

	if rec.prev != c.zero {
		prev := c.data[rec.prev]
		prev.next = rec.next
		c.data[rec.prev] = prev
	}

	if rec.next != c.zero {
		next := c.data[rec.next]
		next.prev = rec.prev
		c.data[rec.next] = next
	}

	return nil
}

// cleanup removes outdated records.
func (c *MapTTLCache[K, V]) cleanup() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	key := c.head
	for {
		rec, ok := c.data[key]
		if !ok {
			break
		}

		if c.now().Sub(rec.timestamp) < c.ttl {
			break
		}

		c.head = rec.next
		delete(c.data, key)

		if key == c.tail {
			c.tail = c.zero
			return nil
		}

		next, ok := c.data[rec.next]
		if ok {
			next.prev = c.zero
			c.data[rec.next] = next
		}
		key = rec.next
	}

	return nil
}
