package geche

import (
	"errors"
	"sync"
)

// UpdateFn is a type for a function to be called to get updated value
// when Updater has a cache miss.
type UpdateFn[K comparable, V any] func(key K) (V, error)

// Updater is a wrapper on any Geche interface implementation
// That calls cache update function if key does not exist in the cache.
// It only allows one Update function per key to be running at a single point of time,
// reducing odds to get a "cache centipede" situation.
type Updater[K comparable, V any] struct {
	cache    Geche[K, V]
	updateFn UpdateFn[K, V]
	pool     chan struct{}
	inFlight map[K]chan struct{}
	mux      sync.RWMutex
}

// NewCacheUpdater returns cache wrapped with Updater. It calls updateFn
// whenever Get function returns ErrNotFound to update cache key.
// Only one updateFn for a given key can run at the same time, and only
// poolSize updateFn with different keys san run simultaneously.
func NewCacheUpdater[K comparable, V any](
	cache Geche[K, V],
	updateFn UpdateFn[K, V],
	poolSize int,
) *Updater[K, V] {
	u := Updater[K, V]{
		cache:    cache,
		updateFn: updateFn,
		pool:     make(chan struct{}, poolSize),
		inFlight: make(map[K]chan struct{}, poolSize),
	}

	return &u
}

// checkAndWaitInFlight waits for other cache key update
// operation to finish. Returns true if had to wait (update operation
// for key was running).
func (u *Updater[K, V]) waitInFlight(key K) bool {
	u.mux.RLock()
	ch, ok := u.inFlight[key]
	u.mux.RUnlock()

	if !ok {
		return false
	}

	<-ch // Wait for channel to be closed.
	return true
}

func (u *Updater[K, V]) Set(key K, value V) {
	u.cache.Set(key, value)
}

func (u *Updater[K, V]) SetIfPresent(key K, value V) (V, bool) {
	return u.cache.SetIfPresent(key, value)
}

func (u *Updater[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	return u.cache.SetIfAbsent(key, value)
}

// Get returns value from the cache. If the value is not in the cache,
// it calls updateFn to get the value and update the cache first.
// Since updateFn can return error, Get is not guaranteed to always return the value.
// When cache update fails, Get will return the error that updateFn returned,
// and not ErrNotFound.
func (u *Updater[K, V]) Get(key K) (V, error) {
	v, err := u.cache.Get(key)
	// Cache miss - update the cache!
	if errors.Is(err, ErrNotFound) {
	wait:
		if u.waitInFlight(key) {
			// If we had to wait, then other goroutine has already updated
			// the cache. Returning it.
			return u.cache.Get(key)
		}

		// Put token in the pool. Will wait if pool is full.
		u.pool <- struct{}{}

		u.mux.Lock()
		// Another goroutine could have started doing update between waitInFlight and here.
		_, ok := u.inFlight[key]
		if ok {
			<-u.pool
			u.mux.Unlock()
			goto wait
		}

		inFlightCh := make(chan struct{})
		u.inFlight[key] = inFlightCh
		u.mux.Unlock()
		defer func() {
			// When finished cache update, releasing all locks.
			u.mux.Lock()
			close(inFlightCh)
			delete(u.inFlight, key)
			u.mux.Unlock()
			<-u.pool
		}()

		v, err = u.updateFn(key)
		if err != nil {
			return v, err
		}

		u.cache.Set(key, v)
	}

	return v, err
}

// Del deletes key from the cache.
func (u *Updater[K, V]) Del(key K) error {
	return u.cache.Del(key)
}

// Snapshot returns a shallow copy of the cache.
// It locks the cache from modification for the duration of the copy.
func (u *Updater[K, V]) Snapshot() map[K]V {
	return u.cache.Snapshot()
}

// Len returns the number of items in the cache.
func (u *Updater[K, V]) Len() int {
	return u.cache.Len()
}

// ListByPrefix should only be called if underlying cache supports ListByPrefix.
// Otherwise it will panic.
func (u *Updater[K, V]) ListByPrefix(prefix string) ([]V, error) {
	kv, ok := any(u.cache).(listerByPrefix[V])
	if !ok {
		panic("cache does not support ListByPrefix")
	}

	return kv.ListByPrefix(prefix)
}
