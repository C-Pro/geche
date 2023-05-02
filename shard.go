package geche

import (
	"math"
	"runtime"
)

// Mapper maps keys to shards. Good mapper maps them uniformly.
type Mapper[K any] interface {
	Map(key K, numShards int) int
}

// StringMapper is a simple implementation mapping string keys to N shards.
// It works best with number of shards that is power of 2, and it
// works up to 256 shards.
type StringMapper struct {}

// Map key to shard number. Should be uniform enough 🤣
func (sm *StringMapper) Map(key string, numSahrds int) int {
	var s byte
	for i := 0; i < len(key); i++ {
		s ^= key[i]
	}

	return int(s) % numSahrds
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

func (s *Sharded[K, V]) Set(key K, value V) {
	s.shards[s.mapper.Map(key, s.N)].Set(key, value)
}

func (s *Sharded[K, V]) Get(key K) (V, error) {
	return s.shards[s.mapper.Map(key, s.N)].Get(key)
}

func (s *Sharded[K, V]) Del(key K) error {
	return s.shards[s.mapper.Map(key, s.N)].Del(key)
}

// defaultShardNumber returns recommended number of shards for current CPU.
// It is computed as nearest power of two that is equal or greater than
// number of available CPU cores.
func defaultShardNumber() int {
	return 1 << (int(math.Ceil(math.Log2(float64(runtime.NumCPU())))))
}
