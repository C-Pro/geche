package geche

import (
	"sync"
	"sync/atomic"
)

// Locker is a wrapper for any Geche interface implementation,
// that provides Lock() and RLock() methods that return Tx object
// implementing Geche interface.
// Returned object is not a transaction in a sense that it does not
// allow commit/rollback or isolation level higher than READ COMMITTED.
// It only provides a way to do multiple cache operations atomically.
type Locker[K comparable, V any] struct {
	cache Geche[K, V]
	mux   *sync.RWMutex
}

// NewLocker creates a new Locker instance.
func NewLocker[K comparable, V any](
	cache Geche[K, V],
) *Locker[K, V] {
	t := Locker[K, V]{
		cache: cache,
		mux:   &sync.RWMutex{},
	}

	return &t
}

// Tx is a "transaction" object returned by Locker.Lock() and Locker.RLock() methods.
// See Locker for more details.
type Tx[K comparable, V any] struct {
	cache    Geche[K, V]
	mux      *sync.RWMutex
	writable bool
	unlocked int32
}

// Retuns read/write locked cache object.
func (t *Locker[K, V]) Lock() *Tx[K, V] {
	t.mux.Lock()
	return &Tx[K, V]{
		cache:    t.cache,
		mux:      t.mux,
		writable: true,
	}
}

// Retuns read-only locked cache object.
func (t *Locker[K, V]) RLock() *Tx[K, V] {
	t.mux.RLock()
	return &Tx[K, V]{
		cache:    t.cache,
		mux:      t.mux,
		writable: false,
	}
}

// Unlock underlying cache.
func (tx *Tx[K, V]) Unlock() {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("unlocking already unlocked transaction")
	}
	atomic.StoreInt32(&tx.unlocked, 1)
	if tx.writable {
		tx.mux.Unlock()
		return
	}
	tx.mux.RUnlock()
}

// Set key-value pair in the underlying locked cache.
// Will panic if called on RLocked Tx.
func (tx *Tx[K, V]) Set(key K, value V) {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	if !tx.writable {
		panic("cannot set in read-only transaction")
	}
	tx.cache.Set(key, value)
}

func (tx *Tx[K, V]) SetIfPresent(key K, value V) (V, bool) {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	if !tx.writable {
		panic("cannot set in read-only transaction")
	}
	return tx.cache.SetIfPresent(key, value)
}

func (tx *Tx[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	if !tx.writable {
		panic("cannot set in read-only transaction")
	}
	return tx.cache.SetIfAbsent(key, value)
}

// Get value by key from the underlying sharded cache.
func (tx *Tx[K, V]) Get(key K) (V, error) {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	return tx.cache.Get(key)
}

// Del key from the underlying locked cache.
// Will panic if called on RLocked Tx.
func (tx *Tx[K, V]) Del(key K) error {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	if !tx.writable {
		panic("cannot del in read-only transaction")
	}
	return tx.cache.Del(key)
}

// Snapshot returns a shallow copy of the cache data.
func (tx *Tx[K, V]) Snapshot() map[K]V {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	return tx.cache.Snapshot()
}

// Len returns total number of elements in the cache.
func (tx *Tx[K, V]) Len() int {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	return tx.cache.Len()
}

type listerByPrefix[V any] interface {
	ListByPrefix(prefix string) ([]V, error)
}

// ListByPrefix should only be called if underlying cache supports ListByPrefix.
func (tx *Tx[K, V]) ListByPrefix(prefix string) ([]V, error) {
	if atomic.LoadInt32(&tx.unlocked) == 1 {
		panic("cannot use unlocked transaction")
	}
	kv, ok := any(tx.cache).(listerByPrefix[V])
	if !ok {
		panic("cache does not support ListByPrefix")
	}

	return kv.ListByPrefix(prefix)
}
