package geche

import (
	"sync"
)

// Length of key to preallocate in dfs.
// It is not a hard limit, but keys longer than this will cause extra allocations.
const maxKeyLength = 512

type trieNode struct {
	// character
	c byte
	// depth level
	d int

	// Nodes down the tree are stored in a map.
	down map[byte]*trieNode

	// Linked list of nodes on the same level.
	next *trieNode
	prev *trieNode

	// Fastpath to first node on the next level for DFS.
	nextLevelHead *trieNode

	terminal bool
}

// Adds a new node to the linked list and returns the new head (if it has changed).
// If head did not change, return value will be nil.
// Should be called on the head node.
func (n *trieNode) addToList(node *trieNode) *trieNode {
	curr := n
	for {
		if node.c < curr.c {
			node.prev = curr.prev
			node.next = curr
			curr.prev = node
			if node.prev != nil {
				node.prev.next = node
			}

			if curr == n {
				// Head has changed.
				return node
			}

			return nil
		}

		if curr.next == nil {
			// Adding to the end of the list.
			node.prev = curr
			curr.next = node
			return nil
		}

		curr = curr.next
	}
}

// Removes node from the linked list.
// Returns the new head (if it has changed).
// Should be called on the head node.
// Will loop forever if node is not in the list.
func (n *trieNode) removeFromList(c byte) *trieNode {
	curr := n
	for {
		if curr.c == c {
			if curr.prev != nil {
				curr.prev.next = curr.next
			}

			if curr.next != nil {
				curr.next.prev = curr.prev
			}

			if curr.prev == nil {
				// Head has changed.
				return curr.next
			}

			return nil
		}

		curr = curr.next
	}
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
		trie: &trieNode{
			down: make(map[byte]*trieNode),
		},
	}

	return &kv
}

// Set key-value pair while updating the trie.
// Panics if key is empty.
func (kv *KV[V]) Set(key string, value V) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	kv.data.Set(key, value)

	if key == "" {
		kv.trie.terminal = true
		return
	}

	node := kv.trie
	for i := 0; i < len(key); i++ {
		if node.down == nil {
			// Creating new level.
			node.down = make(map[byte]*trieNode)
		}

		next := node.down[key[i]]
		if next == nil {
			// Creating new node.
			next = &trieNode{
				c: key[i],
				d: node.d + 1,
			}
			node.down[key[i]] = next
			if node.nextLevelHead == nil {
				node.nextLevelHead = next
			} else {
				// Adding node to the linked list.
				head := node.nextLevelHead.addToList(next)
				if head != nil {
					node.nextLevelHead = head
				}
			}
		}

		node = next
	}

	node.terminal = true
}

// DFS starts with last node of the key prefix.
func (kv *KV[V]) dfs(node *trieNode, prefix []byte) ([]V, error) {
	res := []V{}
	key := make([]byte, len(prefix), maxKeyLength)
	copy(key, prefix)

	// If last node of the prefix is terminal, add it to the result.
	if node.terminal {
		val, err := kv.data.Get(string(prefix))
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}

	// If the node does not contain any descendants, return.
	if node.nextLevelHead == nil {
		return res, nil
	}

	stack := make([]*trieNode, 0, maxKeyLength)
	stack = append(stack, node.nextLevelHead)
	var (
		top       *trieNode
		prevDepth int
		err       error
		val       V
	)
	for {
		if len(stack) == 0 {
			break
		}

		top = stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if top.d > prevDepth {
			// We have descended to the next level.
			key = append(key, top.c)
		} else if top.d < prevDepth {
			// We have ascended to the previous level.
			key = key[:len(key)-1]
			key[len(key)-1] = top.c
		} else {
			key[len(key)-1] = top.c
		}
		prevDepth = top.d

		if top.terminal {
			val, err = kv.data.Get(string(key))
			if err != nil {
				return nil, err
			}
			res = append(res, val)
		}

		// Appending next node of the level to the stack.
		if top.next != nil {
			stack = append(stack, top.next)
		}

		// Appending next level head to the top of the stack.
		if top.nextLevelHead != nil {
			stack = append(stack, top.nextLevelHead)
		}
	}

	return res, nil
}

func (kv *KV[V]) ListByPrefix(prefix string) ([]V, error) {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	node := kv.trie
	for i := 0; i < len(prefix); i++ {
		next := node.down[prefix[i]]
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
		next := node.down[key[i]]
		if next == nil {
			return nil
		}

		prev = node
		node = next
	}

	node.terminal = false

	if node.nextLevelHead == nil && prev != nil {
		head := prev.nextLevelHead.removeFromList(node.c)
		if head != nil {
			prev.nextLevelHead = head
		}
		delete(prev.down, node.c)
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
