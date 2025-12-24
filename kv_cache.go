package geche

import (
	"iter"
	"sort"
	"sync"
)

type byteSlice interface {
	~string | ~[]byte
}

// trieCacheNode is a compact node for the radix tree.
// It uses a sorted slice of values (not pointers) for children.
// This reduces the number of objects on the heap significantly, reducing GC pressure.
type trieCacheNode struct {
	// b is the path segment this node represents.
	b []byte
	// terminal indicates if this node represents the end of a valid key.
	terminal bool
	// children is a list of child nodes, sorted by the first byte of their 'b' segment.
	children []trieCacheNode
	// index of the value in the values slice of the KVCache.
	// Only valid if terminal is true.
	valueIndex int
}

// KVCache is a container that stores the values ordered by their keys using a trie index.
// It allows in order listing of values by prefix.
type KVCache[K byteSlice, V any] struct {
	values   []V
	freelist []int
	trie     *trieCacheNode
	mux      sync.RWMutex
	zero     V
}

// NewKVCache creates a new KVCache.
func NewKVCache[K byteSlice, V any]() *KVCache[K, V] {
	return &KVCache[K, V]{
		trie: &trieCacheNode{},
	}
}

// Set sets the value for the key.
func (kv *KVCache[K, V]) Set(key K, value V) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	kv.insert(key, value)
}

// SetIfPresent sets the value only if the key already exists.
func (kv *KVCache[K, V]) SetIfPresent(key K, value V) (V, bool) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	if old, found := kv.get(key); found {
		kv.insert(key, value)
		return old, true
	}

	return kv.zero, false
}

func (kv *KVCache[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	old, found := kv.get(key)
	if found {
		return old, false
	}

	kv.insert(key, value)
	return kv.zero, true
}

// Get retrieves a value by key.
func (kv *KVCache[K, V]) Get(key K) (V, error) {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	v, ok := kv.get(key)
	if !ok {
		return kv.zero, ErrNotFound
	}

	return v, nil
}

// Del removes the record by key.
// Return value is always nil.
func (kv *KVCache[K, V]) Del(key string) error {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	_ = kv.delete(key)

	return nil
}

// ListByPrefix returns all values with keys starting with the given prefix.
func (kv *KVCache[K, V]) ListByPrefix(prefix string) ([]V, error) {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	node := kv.trie
	searchKey := []byte(prefix)

	var path []byte

	for len(searchKey) > 0 {
		idx, found := node.findChild(searchKey[0])
		if !found {
			return nil, nil
		}

		// Taking the address of the child is safe here because we hold RLock
		// and we don't modify the slice.
		child := &node.children[idx]

		common := commonPrefixLen(child.b, searchKey)
		path = append(path, child.b[:common]...)

		if common < len(searchKey) {
			if common < len(child.b) {
				return nil, nil
			}
			searchKey = searchKey[common:]
			node = child
		} else {
			// Matched prefix, reconstruct path to current node and descend
			remainingNodeSegment := child.b[common:]
			path = append(path, remainingNodeSegment...)
			return kv.dfs(child, path)
		}
	}

	return kv.dfs(kv.trie, []byte{})
}

// AllByPrefix returns an (read only) iterator over values with keys starting with the given prefix.
// The iterator yields key-value pairs.
// Attempting to modify the cache while iterating will lead to a deadlock.
func (kv *KVCache[K, V]) AllByPrefix(prefix string) iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		kv.mux.RLock()
		defer kv.mux.RUnlock()

		node := kv.trie
		searchKey := []byte(prefix)

		// path is the reconstructed key for the DFS traversal starting node.
		var path []byte

		if len(prefix) > 0 {
			var pathPrefix []byte
			for len(searchKey) > 0 {
				idx, found := node.findChild(searchKey[0])
				if !found {
					return // No keys with this prefix.
				}

				child := &node.children[idx]
				common := commonPrefixLen(child.b, searchKey)

				if common < len(searchKey) {
					if common < len(child.b) {
						// e.g., search "ax", child has "ay...". No match.
						return
					}
					// e.g., search "abc", child has "ab". Continue search in child.
					pathPrefix = append(pathPrefix, child.b...)
					searchKey = searchKey[common:]
					node = child
				} else { // common == len(searchKey)
					// Matched prefix. The node for the next part of the key is `child`.
					// The full path to `child` is `pathPrefix` + `child.b`.
					path = append(pathPrefix, child.b...)
					node = child
					goto start_dfs
				}
			}
			// This is for when the prefix matches a node path exactly
			path = pathPrefix
		}

	start_dfs:
		// 2. Stack-based DFS from the found node.
		if node.terminal {
			if !yield(string(path), kv.values[node.valueIndex]) {
				return
			}
		}

		type stackEntry struct {
			node       *trieCacheNode
			pathLength int
		}

		stack := make([]stackEntry, 0, 64)

		for i := len(node.children) - 1; i >= 0; i-- {
			stack = append(stack, stackEntry{
				node:       &node.children[i],
				pathLength: len(path),
			})
		}

		// Re-use path slice for building paths of descendants
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			path = path[:top.pathLength]
			path = append(path, top.node.b...)

			if top.node.terminal {
				if !yield(string(path), kv.values[top.node.valueIndex]) {
					return
				}
			}

			for i := len(top.node.children) - 1; i >= 0; i-- {
				stack = append(stack, stackEntry{
					node:       &top.node.children[i],
					pathLength: len(path),
				})
			}
		}
	}
}

// Snapshot returns a copy of the cache.
func (kv *KVCache[K, V]) Snapshot() map[string]V {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	res := make(map[string]V, kv.Len())

	seq := kv.AllByPrefix("")
	seq(func(k string, v V) bool {
		res[k] = v
		return true
	})
	return res
}

// Len returns the number of the values in the cache.
func (kv *KVCache[K, V]) Len() int {
	return max(0, len(kv.values)-len(kv.freelist))
}

// --- Internal Trie Helpers ---
// Internal helpers are not thread-safe. Caller must hold appropriate lock.

func (kv *KVCache[K, V]) get(key K) (V, bool) {
	node := kv.trie
	keyBytes := []byte(key)

	for len(keyBytes) > 0 {
		idx, found := node.findChild(keyBytes[0])
		if !found {
			return kv.zero, false
		}

		child := &node.children[idx]
		common := commonPrefixLen(child.b, keyBytes)

		if common != len(child.b) {
			return kv.zero, false
		}

		keyBytes = keyBytes[common:]
		node = child
	}

	if node.terminal {
		return kv.values[node.valueIndex], true
	}

	return kv.zero, false
}

func (kv *KVCache[K, V]) addValue(value V) int {
	if len(kv.freelist) > 0 {
		idx := kv.freelist[len(kv.freelist)-1]
		kv.freelist = kv.freelist[:len(kv.freelist)-1]
		kv.values[idx] = value
		return idx
	}

	kv.values = append(kv.values, value)
	return len(kv.values) - 1
}

func (kv *KVCache[K, V]) insert(key K, value V) {
	node := kv.trie
	keyBytes := []byte(key)

	if len(keyBytes) == 0 {
		if !node.terminal {
			node.valueIndex = kv.addValue(value)
			node.terminal = true
		} else {
			kv.values[node.valueIndex] = value
		}
		return
	}

	for len(keyBytes) > 0 {
		idx, found := node.findChild(keyBytes[0])

		if !found {
			// Create value struct
			newNode := trieCacheNode{
				b:          keyBytes,
				terminal:   true,
				valueIndex: kv.addValue(value),
			}
			node.addChildAt(newNode, idx)
			return
		}

		// We take the address of the child element in the slice.
		// This pointer is stable as long as we don't resize 'node.children'.
		// We only resize 'node.children' when adding a new child to 'node',
		// which we don't do in this loop branch (we already found the child).
		child := &node.children[idx]
		common := commonPrefixLen(child.b, keyBytes)

		// We found exact match of the child node's segment.
		if common == len(child.b) {
			keyBytes = keyBytes[common:]
			node = child
			if len(keyBytes) == 0 {
				// We found the full key, update value if node is terminal,
				// otherwise mark node as terminal and insert value.
				if !node.terminal {
					node.valueIndex = kv.addValue(value)
					node.terminal = true
				} else {
					kv.values[node.valueIndex] = value
				}
				return
			}
			continue
		}

		// Split required.
		origSuffix := child.b[common:]
		newSuffix := keyBytes[common:]

		// Create a node representing the rest of the original child.
		// We copy the children slice from the original child.
		restNode := trieCacheNode{
			b:          origSuffix,
			children:   child.children,
			terminal:   child.terminal,
			valueIndex: child.valueIndex,
		}

		// Reset current child to be the branch.
		child.b = child.b[:common]
		child.children = nil // Release the old slice (ownership moved to restNode)
		child.terminal = false

		child.addChild(restNode)

		if len(newSuffix) == 0 {
			child.terminal = true
			child.valueIndex = kv.addValue(value)
		} else {
			newNode := trieCacheNode{
				b:          newSuffix,
				terminal:   true,
				valueIndex: kv.addValue(value),
			}
			child.addChild(newNode)
		}
		return
	}
}

func (kv *KVCache[K, V]) deleteValueAtIndex(idx int) {
	// Clear the value, so if it is a pointer or contains pointers,
	// GC can collect the memory.
	kv.values[idx] = kv.zero
	// Add index to the freelist, so it can be reused.
	kv.freelist = append(kv.freelist, idx)
}

func (kv *KVCache[K, V]) delete(key string) error {
	keyBytes := []byte(key)

	// Stack-based deletion to avoid recursion
	type stackEntry struct {
		node     *trieCacheNode
		keyPart  []byte
		childIdx int // index of child to check, -1 if not yet determined
		parent   *trieCacheNode
	}

	stack := []stackEntry{{
		node:     kv.trie,
		keyPart:  keyBytes,
		childIdx: -1,
		parent:   nil,
	}}

	// Track path for cleanup phase
	type pathEntry struct {
		node     *trieCacheNode
		parent   *trieCacheNode
		childIdx int
	}
	var path []pathEntry

	// Phase 1: Navigate to the target node
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if len(top.keyPart) == 0 {
			// Reached the target node
			if top.node.terminal {
				top.node.terminal = false
				kv.deleteValueAtIndex(top.node.valueIndex)

				// Phase 2: Cleanup - walk back and remove childless non-terminal nodes
				for i := len(path) - 1; i >= 0; i-- {
					node := path[i].node
					parent := path[i].parent
					childIdx := path[i].childIdx

					if len(node.children) == 0 && !node.terminal {
						// Case 1: Delete empty non-terminal node
						// Remove child from slice
						copy(parent.children[childIdx:], parent.children[childIdx+1:])
						parent.children[len(parent.children)-1] = trieCacheNode{}
						parent.children = parent.children[:len(parent.children)-1]
					} else if len(node.children) == 1 && !node.terminal {
						// Case 2: Merge node with its single child
						child := node.children[0]

						node.b = append(node.b, child.b...)
						node.terminal = child.terminal
						node.valueIndex = child.valueIndex
						node.children = child.children
					} else {
						// Node is stable (has >1 children or is terminal), stop cleanup
						break
					}
				}
			}
			return nil
		}

		idx, found := top.node.findChild(top.keyPart[0])
		if !found {
			// Key doesn't exist, nothing to delete
			return nil
		}

		child := &top.node.children[idx]
		common := commonPrefixLen(child.b, top.keyPart)

		if common != len(child.b) {
			// Partial match, key doesn't exist
			return nil
		}

		// Record path for cleanup
		path = append(path, pathEntry{
			node:     child,
			parent:   top.node,
			childIdx: idx,
		})

		// Continue with remaining key
		stack = append(stack, stackEntry{
			node:     child,
			keyPart:  top.keyPart[common:],
			childIdx: -1,
			parent:   top.node,
		})
	}

	return nil
}

func (kv *KVCache[K, V]) dfs(node *trieCacheNode, currentPath []byte) ([]V, error) {
	var res []V

	if node.terminal {
		res = append(res, kv.values[node.valueIndex])
	}

	// If the node has no children, return early
	if len(node.children) == 0 {
		return res, nil
	}

	// Stack-based DFS to avoid recursion
	type stackEntry struct {
		node       *trieCacheNode
		pathLength int // length of path before this node
	}

	stack := make([]stackEntry, 0, maxKeyLength)

	// Push all children of the starting node in reverse order
	for i := len(node.children) - 1; i >= 0; i-- {
		stack = append(stack, stackEntry{
			node:       &node.children[i],
			pathLength: len(currentPath),
		})
	}

	for len(stack) > 0 {
		// Pop from stack
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Restore path to parent length and append current node's segment
		currentPath = currentPath[:top.pathLength]
		currentPath = append(currentPath, top.node.b...)

		if top.node.terminal {
			res = append(res, kv.values[top.node.valueIndex])
		}

		// Push children in reverse order so they are processed in correct order
		for i := len(top.node.children) - 1; i >= 0; i-- {
			stack = append(stack, stackEntry{
				node:       &top.node.children[i],
				pathLength: len(currentPath),
			})
		}
	}

	return res, nil
}

func (n *trieCacheNode) findChild(c byte) (int, bool) {
	idx := sort.Search(len(n.children), func(i int) bool {
		return n.children[i].b[0] >= c
	})
	if idx < len(n.children) && n.children[idx].b[0] == c {
		return idx, true
	}
	return idx, false
}

func (n *trieCacheNode) addChild(child trieCacheNode) {
	idx, _ := n.findChild(child.b[0])
	n.addChildAt(child, idx)
}

func (n *trieCacheNode) addChildAt(child trieCacheNode, idx int) {
	n.children = append(n.children, trieCacheNode{})
	copy(n.children[idx+1:], n.children[idx:])
	n.children[idx] = child
}
