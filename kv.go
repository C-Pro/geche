package geche

import (
	"sync"
)

type trieNode struct {
	c    byte
	next map[byte]*trieNode
	// min and max are used to speed up the search
	// without resorting to implementing double linked list.
	min byte
	max byte

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
			node.min = key[i]
			node.max = key[i]
		}

		if key[i] < node.min {
			node.min = key[i]
		}
		if key[i] > node.max {
			node.max = key[i]
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

type stackItem[V any] struct {
	node   *trieNode
	prefix []byte
	c      byte
}

func (kv *KV[V]) dfs(node *trieNode, prefix []byte) ([]V, error) {
	res := []V{}

	stack := []stackItem[V]{
		{node: node, c: node.min},
	}

	if node.terminal {
		val, err := kv.data.Get(string(prefix))
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}

	var (
		top  stackItem[V]
		next *trieNode
	)
	for {
		if len(stack) == 0 {
			break
		}

		top = stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if top.c < top.node.max {
			stack = append(stack, stackItem[V]{node: top.node, prefix: top.prefix, c: top.c + 1})
		}

		if top.node.next[top.c] != nil {
			next = top.node.next[top.c]
			if next.terminal {
				key := append(prefix, top.prefix...)
				val, err := kv.data.Get(string(append(key, top.c)))
				if err != nil {
					return nil, err
				}
				res = append(res, val)
			}
			stack = append(stack, stackItem[V]{node: next, prefix: append(top.prefix, top.c), c: top.node.min})
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
