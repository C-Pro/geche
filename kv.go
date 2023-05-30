package geche

import (
	"sync"
)

type trieNode struct {
	c        byte
	next     map[byte]*trieNode
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

	node := kv.trie
	prev := node
	for i := 0; i < len(key); i++ {
		if node.next == nil {
			node.next = make(map[byte]*trieNode)
		}

		node = node.next[key[i]]
		if node == nil {
			node = &trieNode{
				c: key[i],
			}
			prev.next[key[i]] = node
		}

		prev = node
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

	i := byte(0)
	for {
		if node.next[i] != nil {
			next := node.next[i]
			nextRes, err := kv.dfs(next, append(prefix, next.c))
			if err != nil {
				return nil, err
			}
			res = append(res, nextRes...)
		}

		if i == 255 {
			break
		}
		i++
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

// Get value by key from the underlying cache.
func (kv *KV[V]) Get(key string) (V, error) {
	return kv.data.Get(key)
}

// Del key from the underlying cache.
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
	i := byte(0)
	for {
		if node.next[i] != nil {
			empty = false
			break
		}

		if i == 255 {
			break
		}
		i++
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

// Len returns total number of elements in the underlying caches.
func (kv *KV[V]) Len() int {
	return kv.data.Len()
}
