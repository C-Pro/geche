package geche

import (
	"math"
	"runtime"
)

type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Mapper maps keys to shards. Good mapper maps them uniformly.
type Mapper[K any] interface {
	Map(key K, numShards int) int
}

// NumberMapper maps integer keys to N shards using modulo operation.
type NumberMapper[K integer] struct{}

// Map key to shard number.
func (nm *NumberMapper[K]) Map(key K, numShards int) int {
	return int(uint64(key) % uint64(numShards))
}

// StringMapper is a simple implementation mapping string keys to N shards.
// It works best with number of shards that is power of 2, and it
// works up to 256 shards.
type StringMapper struct{}

// Map key to shard number. Should be uniform enough 🤣
func (sm *StringMapper) Map(key string, numShards int) int {
	var s byte
	for i := 0; i < len(key); i++ {
		s ^= key[i]
	}

	return int(s) % numShards
}

// Sharded is a wrapper for any Geche interface implementation
// that provides the same interface itself. The idea is to better
// utilize CPUs using several thread safe shards.
type Sharded[K comparable, V any] struct {
	N      int
	shards []Geche[K, V]
	mapper Mapper[K]
}

// NewSharded creates numShards underlying cache containers
// using shardFactory function to initialize each,
// and returns Sharded instance that implements Geche interface
// and routes operations to shards using keyMapper function.
func NewSharded[K comparable, V any](
	shardFactory func() Geche[K, V],
	numShards int,
	keyMapper Mapper[K],
) *Sharded[K, V] {
	if numShards <= 0 {
		numShards = defaultShardNumber()
	}
	s := Sharded[K, V]{
		N:      numShards,
		mapper: keyMapper,
	}

	for i := 0; i < numShards; i++ {
		s.shards = append(s.shards, shardFactory())
	}

	return &s
}

// Set key-value pair in the underlying sharded cache.
func (s *Sharded[K, V]) Set(key K, value V) {
	s.shards[s.mapper.Map(key, s.N)].Set(key, value)
}

func (s *Sharded[K, V]) SetIfPresent(key K, value V) (V, bool) {
	return s.shards[s.mapper.Map(key, s.N)].SetIfPresent(key, value)
}

func (s *Sharded[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	return s.shards[s.mapper.Map(key, s.N)].SetIfAbsent(key, value)
}

// Get value by key from the underlying sharded cache.
func (s *Sharded[K, V]) Get(key K) (V, error) {
	return s.shards[s.mapper.Map(key, s.N)].Get(key)
}

// Del key from the underlying sharded cache.
func (s *Sharded[K, V]) Del(key K) error {
	return s.shards[s.mapper.Map(key, s.N)].Del(key)
}

// Snapshot returns a shallow copy of the cache data.
// Sequentially locks each of she undelnying shards
// from modification for the duration of the copy.
func (s *Sharded[K, V]) Snapshot() map[K]V {
	snapshot := make(map[K]V)
	for _, shard := range s.shards {
		for k, v := range shard.Snapshot() {
			snapshot[k] = v
		}
	}

	return snapshot
}

// defaultShardNumber returns recommended number of shards for current CPU.
// It is computed as nearest power of two that is equal or greater than
// number of available CPU cores.
func defaultShardNumber() int {
	return 1 << (int(math.Ceil(math.Log2(float64(runtime.NumCPU())))))
}

// Len returns total number of elements in the underlying sharded caches.
func (s *Sharded[K, V]) Len() int {
	var l int
	for _, shard := range s.shards {
		l += shard.Len()
	}

	return l
}
