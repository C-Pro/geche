package geche

import (
	"bytes"
	"sync"
)

// Length of key to preallocate in dfs.
// It is not a hard limit, but keys longer than this will cause extra allocations.
const maxKeyLength = 512

type trieNode struct {
	// Node suffix. Single byte for most nodes, but can be longer for tail node.
	b []byte
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
		if node.b[0] < curr.b[0] {
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
// Returns true if the list is now empty.
// Should be called on the head node.
// Will loop forever if node is not in the list.
func (n *trieNode) removeFromList(c byte) (*trieNode, bool) {
	curr := n
	for {
		if curr.b[0] == c {
			if curr.prev != nil {
				curr.prev.next = curr.next
			}

			if curr.next != nil {
				curr.next.prev = curr.prev
			}

			if curr.prev == nil {
				// Head has changed.
				if curr.next == nil {
					// List is now empty.
					return nil, true
				}
				return curr.next, false
			}

			return nil, false
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

func (kv *KV[V]) SetIfPresent(key string, value V) (V, bool) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	previousVal, err := kv.data.Get(key)
	if err == nil {
		kv.set(key, value)
		return previousVal, true
	}

	return previousVal, false
}

func (kv *KV[V]) SetIfAbsent(key string, value V) (V, bool) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	previousVal, err := kv.data.Get(key)
	if err == nil {
		return previousVal, false
	}

	kv.set(key, value)
	return previousVal, true
}

// Set key-value pair while updating the trie.
// Panics if key is empty.
func (kv *KV[V]) Set(key string, value V) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	kv.set(key, value)
}

func commonPrefixLen(a, b []byte) int {
	i := 0
	for ; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}

	return i
}

// Depth First Search starts with last node of the key prefix and traverses the trie,
// appending all terminal nodes to the result.
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

	// Instead of recursive DFS, we use stack-based approach.
	stack := make([]*trieNode, 0, maxKeyLength)
	stack = append(stack, node.nextLevelHead)
	var (
		top       *trieNode
		prevDepth int
		err       error
		val       V
	)
	for len(stack) > 0 {
		// Pop the top node from the stack.
		top = stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if top.d > prevDepth {
			// We have descended to the next level.
			key = append(key, top.b...)
		} else if top.d < prevDepth {
			// We have ascended to the previous level.
			key = key[:top.d]
			key[len(key)-1] = top.b[0]
			if len(top.b) > 1 {
				key = append(key, top.b[1:]...)
			}
		} else {
			key = key[:top.d]
			key[len(key)-1] = top.b[0]
			if len(top.b) > 1 {
				key = append(key, top.b[1:]...)
			}
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
		// If we reached a multibyte tail node, we can return its value,
		// since tail nodes have no descendants.
		if len(next.b) > 1 && len(next.b) >= len(prefix)-i {
			if bytes.Equal(next.b[:len(prefix)-i], []byte(prefix)[i:]) {
				v, err := kv.data.Get(prefix + string(next.b[len(prefix)-i:]))
				return []V{v}, err
			}
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
	stack := []*trieNode{}
	found := false
	for i := 0; i < len(key); i++ {
		next := node.down[key[i]]
		if next == nil {
			// If we are here, the key does not exist.
			return kv.data.Del(key)
		}

		stack = append(stack, node)
		node = next
		if bytes.Equal(node.b, []byte(key)[i:]) {
			if node.terminal {
				found = true
			}
			break
		}
	}

	if !found {
		// If we are here, the key does not exist.
		return kv.data.Del(key)
	}

	node.terminal = false

	// Go back the stack removing nodes with no descendants.
	for i := len(stack) - 1; i >= 0; i-- {
		prev := stack[i]
		stack = stack[:i]
		if node.nextLevelHead == nil {
			head, empty := prev.nextLevelHead.removeFromList(node.b[0])
			if head != nil || empty {
				prev.nextLevelHead = head
			}
			delete(prev.down, node.b[0])
		}

		if prev.terminal || len(prev.down) > 0 && prev == kv.trie {
			break
		}

		node = prev
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

func (kv *KV[V]) set(key string, value V) {
	kv.data.Set(key, value)

	if key == "" {
		kv.trie.terminal = true
		return
	}

	keyb := []byte(key)
	node := kv.trie
	for len(keyb) > 0 {
		if node.down == nil {
			// Creating new level.
			node.down = make(map[byte]*trieNode)
		}

		next := node.down[keyb[0]]
		if next == nil {
			// Creating new node.
			next = &trieNode{
				b: keyb,
				d: node.d + 1,
			}
			node.down[keyb[0]] = next
			if node.nextLevelHead == nil {
				node.nextLevelHead = next
			} else {
				// Adding node to the linked list.
				head := node.nextLevelHead.addToList(next)
				if head != nil {
					node.nextLevelHead = head
				}
			}
		} else if len(next.b) == 1 {
			// Single byte nodes are a simple case.
		} else {
			// Multi byte nodes require splitting.

			// Removing node from the linked list.
			head, empty := node.nextLevelHead.removeFromList(keyb[0])
			if empty {
				node.nextLevelHead = nil
			} else if head != nil {
				node.nextLevelHead = head
			}

			commonPrefixLen := commonPrefixLen(keyb, next.b)
			for i := 0; i < commonPrefixLen; i++ {
				// Creating new single-byte node.
				newNode := &trieNode{
					b:    []byte{keyb[i]},
					d:    node.d + 1,
					down: make(map[byte]*trieNode),
				}
				node.down[keyb[i]] = newNode
				if node.nextLevelHead == nil {
					node.nextLevelHead = newNode
				} else {
					head := node.nextLevelHead.addToList(newNode)
					if head != nil {
						node.nextLevelHead = head
					}
				}

				node = newNode
			}

			if (bytes.Equal(next.b, keyb[:commonPrefixLen]) && next.terminal) || len(keyb) == commonPrefixLen {
				// If last node is end of key, or end of the node we are splitting, mark it as terminal.
				node.terminal = true
			}

			// Adding removed node back.
			if len(next.b) > commonPrefixLen {
				// Creating new suffix (potentially multi-byte) node.
				newNode := &trieNode{
					b:        next.b[commonPrefixLen:],
					d:        node.d + 1,
					terminal: true,
				}
				node.down[next.b[commonPrefixLen]] = newNode
				node.nextLevelHead = newNode
			}

			// Adding new tail node.
			if len(keyb) > commonPrefixLen {
				// Creating new suffix (potentially multi-byte) node.
				newNode := &trieNode{
					b:        keyb[commonPrefixLen:],
					d:        node.d + 1,
					terminal: true,
				}
				node.down[keyb[commonPrefixLen]] = newNode
				if node.nextLevelHead == nil {
					node.nextLevelHead = newNode
				} else {
					head := node.nextLevelHead.addToList(newNode)
					if head != nil {
						node.nextLevelHead = head
					}
				}
			}

			// keyb = keyb[commonPrefixLen:]
			// continue
			return
		}

		keyb = keyb[commonPrefixLen(keyb, next.b):]
		node = next
	}

	node.terminal = true
}
