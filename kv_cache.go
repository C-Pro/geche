package geche

import (
	"iter"
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
	searchKey := prefix

	for len(searchKey) > 0 {
		idx, found := node.findChild(searchKey[0])
		if !found {
			return nil, nil
		}

		// Taking the address of the child is safe here because we hold RLock
		// and we don't modify the slice.
		child := &node.children[idx]

		var common int
		if len(child.b) == 1 {
			common = 1
		} else {
			common = commonPrefixLenStr(child.b, searchKey)
		}

		if common < len(searchKey) {
			if common < len(child.b) {
				return nil, nil
			}
			searchKey = searchKey[common:]
			node = child
		} else {
			return kv.dfs(child)
		}
	}

	return kv.dfs(kv.trie)
}

// AllByPrefix returns an (read only) iterator over values with keys starting with the given prefix.
// The iterator yields key-value pairs.
// Attempting to modify the cache while iterating will lead to a deadlock.
// Iterator holds RLock on the cache for the whole iteration, so it is not recommended
// to use it for long-running loops if cache can be accessed concurrently.
func (kv *KVCache[K, V]) AllByPrefix(prefix string) iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		kv.mux.RLock()
		defer kv.mux.RUnlock()

		node := kv.trie

		// path is the reconstructed key for the DFS traversal starting node.
		var path []byte

		if len(prefix) > 0 {
			searchKey := []byte(prefix)
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
					break
				}
			}
		}

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

	res := make(map[string]V, kv.len())

	seq := kv.AllByPrefix("")
	seq(func(k string, v V) bool {
		res[k] = v
		return true
	})
	return res
}

func (kv *KVCache[K, V]) len() int {
	return max(0, len(kv.values)-len(kv.freelist))
}

// Len returns the number of the values in the cache.
func (kv *KVCache[K, V]) Len() int {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	return kv.len()
}

// Clear removes all elements from the cache while preserving allocated capacities.
func (kv *KVCache[K, V]) Clear() {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	clear(kv.values)
	kv.values = kv.values[:0]

	clear(kv.freelist)
	kv.freelist = kv.freelist[:0]

	clear(kv.trie.children)
	kv.trie.children = kv.trie.children[:0]
	kv.trie.terminal = false
	kv.trie.valueIndex = 0
}

// --- Internal Trie Helpers ---
// Internal helpers are not thread-safe. Caller must hold appropriate lock.

func (kv *KVCache[K, V]) get(key K) (V, bool) {
	node := kv.trie

	switch k := any(key).(type) {
	case string:
		keyStr := k
		for len(keyStr) > 0 {
			idx, found := node.findChild(keyStr[0])
			if !found {
				return kv.zero, false
			}

			child := &node.children[idx]
			var common int
			if len(child.b) == 1 {
				common = 1
			} else {
				common = commonPrefixLenStr(child.b, keyStr)
				if common != len(child.b) {
					return kv.zero, false
				}
			}

			keyStr = keyStr[common:]
			node = child
		}
	case []byte:
		keyBytes := k
		for len(keyBytes) > 0 {
			idx, found := node.findChild(keyBytes[0])
			if !found {
				return kv.zero, false
			}

			child := &node.children[idx]
			var common int
			if len(child.b) == 1 {
				common = 1
			} else {
				common = commonPrefixLen(child.b, keyBytes)
				if common != len(child.b) {
					return kv.zero, false
				}
			}

			keyBytes = keyBytes[common:]
			node = child
		}
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

	switch k := any(key).(type) {
	case string:
		keyStr := k
		if len(keyStr) == 0 {
			if !node.terminal {
				node.valueIndex = kv.addValue(value)
				node.terminal = true
			} else {
				kv.values[node.valueIndex] = value
			}
			return
		}

		for len(keyStr) > 0 {
			idx, found := node.findChild(keyStr[0])

			if !found {
				newNode := trieCacheNode{
					b:          []byte(keyStr),
					terminal:   true,
					valueIndex: kv.addValue(value),
				}
				node.addChildAt(newNode, idx)
				return
			}

			child := &node.children[idx]
			var common int
			if len(child.b) == 1 {
				common = 1
			} else {
				common = commonPrefixLenStr(child.b, keyStr)
			}

			// We found exact match of the child node's segment.
			if common == len(child.b) {
				keyStr = keyStr[common:]
				node = child
				if len(keyStr) == 0 {
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
			newSuffix := keyStr[common:]

			restNode := trieCacheNode{
				b:          origSuffix,
				children:   child.children,
				terminal:   child.terminal,
				valueIndex: child.valueIndex,
			}

			child.b = child.b[:common]
			child.terminal = false

			if len(newSuffix) == 0 {
				child.terminal = true
				child.valueIndex = kv.addValue(value)
				child.children = []trieCacheNode{restNode}
			} else {
				newNode := trieCacheNode{
					b:          []byte(newSuffix),
					terminal:   true,
					valueIndex: kv.addValue(value),
				}
				if origSuffix[0] < newSuffix[0] {
					child.children = []trieCacheNode{restNode, newNode}
				} else {
					child.children = []trieCacheNode{newNode, restNode}
				}
			}
			return
		}

	case []byte:
		keyBytes := k
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
				bCopy := make([]byte, len(keyBytes))
				copy(bCopy, keyBytes)
				newNode := trieCacheNode{
					b:          bCopy,
					terminal:   true,
					valueIndex: kv.addValue(value),
				}
				node.addChildAt(newNode, idx)
				return
			}

			child := &node.children[idx]
			var common int
			if len(child.b) == 1 {
				common = 1
			} else {
				common = commonPrefixLen(child.b, keyBytes)
			}

			// We found exact match of the child node's segment.
			if common == len(child.b) {
				keyBytes = keyBytes[common:]
				node = child
				if len(keyBytes) == 0 {
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

			restNode := trieCacheNode{
				b:          origSuffix,
				children:   child.children,
				terminal:   child.terminal,
				valueIndex: child.valueIndex,
			}

			child.b = child.b[:common]
			child.terminal = false

			if len(newSuffix) == 0 {
				child.terminal = true
				child.valueIndex = kv.addValue(value)
				child.children = []trieCacheNode{restNode}
			} else {
				bCopy := make([]byte, len(newSuffix))
				copy(bCopy, newSuffix)
				newNode := trieCacheNode{
					b:          bCopy,
					terminal:   true,
					valueIndex: kv.addValue(value),
				}
				if origSuffix[0] < newSuffix[0] {
					child.children = []trieCacheNode{restNode, newNode}
				} else {
					child.children = []trieCacheNode{newNode, restNode}
				}
			}
			return
		}
	}
}

func (kv *KVCache[K, V]) deleteValueAtIndex(idx int) {
	kv.values[idx] = kv.zero
	kv.freelist = append(kv.freelist, idx)
}

func (kv *KVCache[K, V]) delete(key string) error {
	type pathEntry struct {
		node     *trieCacheNode
		parent   *trieCacheNode
		childIdx int
	}
	var path []pathEntry

	node := kv.trie
	keyPart := key

	for {
		if len(keyPart) == 0 {
			if node.terminal {
				node.terminal = false
				kv.deleteValueAtIndex(node.valueIndex)

				for i := len(path) - 1; i >= 0; i-- {
					pNode := path[i].node
					pParent := path[i].parent
					childIdx := path[i].childIdx

					if len(pNode.children) == 0 && !pNode.terminal {
						copy(pParent.children[childIdx:], pParent.children[childIdx+1:])
						pParent.children[len(pParent.children)-1] = trieCacheNode{}
						pParent.children = pParent.children[:len(pParent.children)-1]
					} else if len(pNode.children) == 1 && !pNode.terminal {
						child := pNode.children[0]

						pNode.b = append(pNode.b, child.b...)
						pNode.terminal = child.terminal
						pNode.valueIndex = child.valueIndex
						pNode.children = child.children
					} else {
						break
					}
				}
			}
			return nil
		}

		idx, found := node.findChild(keyPart[0])
		if !found {
			return nil
		}

		child := &node.children[idx]
		var common int
		if len(child.b) == 1 {
			common = 1
		} else {
			common = commonPrefixLenStr(child.b, keyPart)
			if common != len(child.b) {
				return nil
			}
		}

		path = append(path, pathEntry{
			node:     child,
			parent:   node,
			childIdx: idx,
		})

		node = child
		keyPart = keyPart[common:]
	}
}

func (kv *KVCache[K, V]) dfs(node *trieCacheNode) ([]V, error) {
	var res []V

	if node.terminal {
		res = append(res, kv.values[node.valueIndex])
	}

	if len(node.children) == 0 {
		return res, nil
	}

	type stackEntry struct {
		node *trieCacheNode
	}

	stack := make([]stackEntry, 0, maxKeyLength)

	for i := len(node.children) - 1; i >= 0; i-- {
		stack = append(stack, stackEntry{
			node: &node.children[i],
		})
	}

	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if top.node.terminal {
			res = append(res, kv.values[top.node.valueIndex])
		}

		for i := len(top.node.children) - 1; i >= 0; i-- {
			stack = append(stack, stackEntry{
				node: &top.node.children[i],
			})
		}
	}

	return res, nil
}

func (n *trieCacheNode) findChild(c byte) (int, bool) {
	for i := 0; i < len(n.children); i++ {
		first := n.children[i].b[0]
		if first == c {
			return i, true
		}
		if first > c {
			return i, false
		}
	}
	return len(n.children), false
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

func commonPrefixLenStr(a []byte, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
