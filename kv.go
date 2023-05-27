package geche

import (
	"sync"
)

type trieNode struct {
	c        byte
	next     [256]*trieNode
	terminal bool
}

type KV[V any] struct {
	data Geche[string, V]
	trie *trieNode
	mux  sync.RWMutex
}

func NewKV[V any](
	cache Geche[string, V],
) *KV[V] {
	kv := KV[V]{
		data: cache,
		trie: new(trieNode),
	}

	return &kv
}

// Set key-value pair while updating the trie.
func (kv *KV[V]) Set(key string, value V) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	kv.data.Set(key, value)
	if key == "" {
		return
	}

	node := kv.trie
	for i := 0; i < len(key); i++ {
		next := node.next[key[i]]
		if next == nil {
			next = &trieNode{
				c: key[i],
			}
			node.next[key[i]] = next
		}

		node = next
	}

	node.terminal = true
}

func (kv *KV[V]) dfs(node *trieNode, prefix []byte) ([]V, error) {
	res := []V{}
	if node.terminal {
		val, err := kv.data.Get(string(prefix))
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}

	for i := 0; i < len(node.next); i++ {
		if node.next[i] != nil {
			next := node.next[i]
			nextRes, err := kv.dfs(next, append(prefix, next.c))
			if err != nil {
				return nil, err
			}
			res = append(res, nextRes...)
		}
	}

	return res, nil
}

func (kv *KV[V]) ListByPrefix(prefix string) ([]V, error) {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	node := kv.trie
	for i := 0; i < len(prefix); i++ {
		next := node.next[prefix[i]]
		if next == nil {
			return nil, nil
		}
		node = next
	}

	return kv.dfs(node, []byte(prefix))
}

// Get value by key from the underlying sharded cache.
func (kv *KV[V]) Get(key string) (V, error) {
	return kv.data.Get(key)
}

// Del key from the underlying sharded cache.
func (kv *KV[V]) Del(key string) error {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	node := kv.trie
	var prev *trieNode
	for i := 0; i < len(key); i++ {
		next := node.next[key[i]]
		if next == nil {
			return nil
		}

		prev = node
		node = next
	}

	node.terminal = false

	empty := true
	for i := 0; i < len(node.next); i++ {
		if node.next[i] != nil {
			empty = false
			break
		}
	}

	if empty {
		prev.next[node.c] = nil
	}

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
