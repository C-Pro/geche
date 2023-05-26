package geche

import (
	"math"
	"runtime"
)

type trieNode struct {
	c byte
	next [256]*trieNode
	terminal bool
}

type KV[V any] struct {
	data Geche[string, V]
	trie *trieNode
}

func NewKV[V any](
	cache Geche[string, V],
) *KV[V] {
	kv := KV[V]{
		data: cache,
	}

	return &kv
}

// Set key-value pair in the underlying sharded cache.
func (kv *KV[V]) Set(key string, value V) {
	kv.data.Set(key, value)
	if key == "" {
		return
	}

	node := kv.trie
	for i := 0; i < len(key); i++ {
		next := node.next[key[i]]
		if next == nil {
			next = new(trieNode)
			node.next[key[i]] = next
		}

		node = next
	}

	node.terminal = true
}


func (kv *KV[V]) GetByPrefix(prefix string) ([]V, error) {
	res := []V{}

	node := kv.trie
	for i := 0; i < len(prefix); i++ {
		next := node.next[prefix[i]]
		if next == nil {
			return res, nil
		}
		node = next
	}

	

	return res, nil
}


// Get value by key from the underlying sharded cache.
func (kv *KV[V]) Get(key string) (V, error) {
	return kv.data.Get(key)
}

// Del key from the underlying sharded cache.
func (kv *KV[V]) Del(key string) error {
	return kv.data.Del(key)
}

// Snapshot returns a shallow copy of the cache data.
// Sequentially locks each of she undelnying shards
// from modification for the duration of the copy.
func (kv *KV[V]) Snapshot() map[string]V {
	return kv.data.Snapshot()
}

// Len returns total number of elements in the underlying sharded caches.
func (kv *KV[V]) Len() int {
	return kv.data.Len()
}
