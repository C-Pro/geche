package geche

import (
	"context"
	"sync"
	"time"
)

// defaultCleanupInterval controls how often cache will purge obsolete values.
const defaultCleanupInterval = time.Second

type record[T any, K comparable] struct {
	// linked list to maintain order
	prev      K
	next      K
	value     T
	timestamp time.Time
}

// zero returns zero value for type K.
func zero[K comparable]() K {
	var z K
	return z
}

// MapTTLCache is the thread-safe map-based cache with TTL support.
type MapTTLCache[T any, K comparable] struct {
	data            map[K]record[T, K]
	mux             sync.RWMutex
	ttl             time.Duration
	now             func() time.Time
	tail            K
	head            K
	zero            K
}

func NewMapTTLCache[T any, K comparable](
	ctx context.Context,
	ttl time.Duration,
	cleanupInterval time.Duration,
	) *MapTTLCache[T, K] {
	if cleanupInterval == 0 {
		cleanupInterval = defaultCleanupInterval
	}
	c := MapTTLCache[T, K]{
		data:            make(map[K]record[T, K]),
		ttl:             ttl,
		now:             time.Now,
		zero:            zero[K](), // cache zero value for comparisons.
	}

	go func(ctx context.Context) {
		t := time.NewTicker(cleanupInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.cleanup()
			}
		}
	}(ctx)

	return &c
}

func (c *MapTTLCache[T, K]) Set(key K, value T) {
	c.mux.Lock()
	defer c.mux.Unlock()

	val := record[T, K]{
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

func (c *MapTTLCache[T, K]) Get(key K) (T, error) {
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

func (c *MapTTLCache[T, K]) Del(key K) error {
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
func (c *MapTTLCache[T, K]) cleanup() error {
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
