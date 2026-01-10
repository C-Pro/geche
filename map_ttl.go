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

type onEvictFunc[K comparable, V any] func(key K, value V)

// MapTTLCache is the thread-safe map-based cache with TTL cache invalidation support.
// MapTTLCache uses double linked list to maintain FIFO order of inserted values.
type MapTTLCache[K comparable, V any] struct {
	data    map[K]ttlRec[K, V]
	mux     sync.RWMutex
	ttl     time.Duration
	// TODO: replace with sync.Test
	now     func() time.Time
	onEvict onEvictFunc[K, V]
	tail    K
	head    K
	zero    K
}

// NewMapTTLCache creates MapTTLCache instance and spawns background
// cleanup goroutine, that periodically removes outdated records.
// Cleanup goroutine will run cleanup once in cleanupInterval until ctx is canceled.
// Each record in the cache is valid for ttl duration since it was Set.
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

// OnEvict sets a callback function that will be called when an entry is evicted from the cache
// due to TTL expiration. The callback receives the key and value of the evicted entry.
// Note that the eviction callback is not called for Del operation.
func (c *MapTTLCache[K, V]) OnEvict(f onEvictFunc[K, V]) {
	c.mux.Lock()
	c.onEvict = f
	c.mux.Unlock()
}

func (c *MapTTLCache[K, V]) Set(key K, value V) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.set(key, value)
}

// SetIfPresent sets the given key to the given value if the key was already present, and resets the TTL
func (c *MapTTLCache[K, V]) SetIfPresent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	old, err := c.get(key)
	if err == nil {
		c.set(key, value)
		return old, true
	}

	return old, false
}

func (c *MapTTLCache[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	old, err := c.get(key)
	if err == nil {
		return old, false
	}

	c.set(key, value)
	return old, true
}

// Get returns ErrNotFound if key is not found in the cache or record is outdated.
func (c *MapTTLCache[K, V]) Get(key K) (V, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return c.get(key)
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

// cleanup removes outdated records
// and calls eviction callbacks.
func (c *MapTTLCache[K, V]) cleanup() error {
	var (
		evicted map[K]V
		onEvict onEvictFunc[K, V]
	)

	c.mux.Lock()

	// Preallocate a small map for evicted records
	// if eviction callback is set.
	if c.onEvict != nil {
		onEvict = c.onEvict
		evicted = make(map[K]V, 16)
	}

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

		if onEvict != nil {
			evicted[key] = rec.value
		}

		if key == c.tail {
			c.tail = c.zero
			break
		}

		next, ok := c.data[rec.next]
		if ok {
			next.prev = c.zero
			c.data[rec.next] = next
		}
		key = rec.next
	}
	c.mux.Unlock()

	// Call eviction callbacks outside of the lock.
	for k, v := range evicted {
		onEvict(k, v)
	}

	return nil
}

// Snapshot returns a shallow copy of the cache data.
// Locks the cache from modification for the duration of the copy.
func (c *MapTTLCache[K, V]) Snapshot() map[K]V {
	c.mux.RLock()
	defer c.mux.RUnlock()

	snapshot := make(map[K]V, len(c.data))
	for k, v := range c.data {
		snapshot[k] = v.value
	}

	return snapshot
}

// Len returns the number of records in the cache.
func (c *MapTTLCache[K, V]) Len() int {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return len(c.data)
}

func (c *MapTTLCache[K, V]) set(key K, value V) {
	ts := c.now()
	val := ttlRec[K, V]{
		value:     value,
		prev:      c.tail,
		timestamp: ts,
	}

	if c.head == c.zero {
		c.head = key
		c.tail = key
		c.data[key] = val
		return
	}

	// If it's already the tail, we only need to update the value and timestamp
	if c.tail == key {
		rec := c.data[c.tail]
		rec.timestamp = ts
		rec.value = value
		c.data[c.tail] = rec
		return
	}

	// If the record for this key already exists
	// and is not already the tail of the list,
	// removing it before adding to the tail.
	if rec, ok := c.data[key]; ok {
		next := c.data[rec.next]

		// edge case: the current head becomes the new tail
		if key == c.head {
			c.head = rec.next
			next.prev = c.zero
		} else {
			prev := c.data[rec.prev]
			prev.next = rec.next
			c.data[rec.prev] = prev
			next.prev = rec.prev
		}

		c.data[rec.next] = next
	}

	tailval := c.data[c.tail]
	tailval.next = key
	c.data[c.tail] = tailval
	c.tail = key
	c.data[key] = val
}

func (c *MapTTLCache[K, V]) get(key K) (V, error) {
	v, ok := c.data[key]
	if !ok {
		return v.value, ErrNotFound
	}

	if c.now().Sub(v.timestamp) >= c.ttl {
		return v.value, ErrNotFound
	}

	return v.value, nil
}
