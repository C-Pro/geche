package geche

import (
	"sort"
	"sync"
)

// trieNode is a compact node for the radix tree.
// It uses a sorted slice for children to minimize memory footprint compared to maps.
type trieNode struct {
	// b is the path segment this node represents.
	b []byte
	// children is a list of child nodes, sorted by the first byte of their 'b' segment.
	children []*trieNode
	// terminal indicates if this node represents the end of a valid key.
	terminal bool
}

// KV is a wrapper that adds ordered prefix listing capabilities to any Geche cache.
// It maintains a trie index alongside the underlying cache.
type KV[V any] struct {
	data Geche[string, V]
	trie *trieNode
	mux  sync.RWMutex
}

// NewKV creates a new KV wrapper.
func NewKV[V any](cache Geche[string, V]) *KV[V] {
	return &KV[V]{
		data: cache,
		trie: &trieNode{},
	}
}

// Set sets the value for the key in the underlying cache and updates the trie index.
func (kv *KV[V]) Set(key string, value V) {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	kv.data.Set(key, value)
	kv.insert(key)
}

// SetIfPresent sets the value only if the key already exists.
// Note: This implementation assumes the key presence in the underlying cache
// implies presence in the trie, so it does not modify the trie structure.
func (kv *KV[V]) SetIfPresent(key string, value V) (V, bool) {
	// We don't need to touch the trie if the key exists; just update value.
	// If the key doesn't exist in data, we do nothing.
	return kv.data.SetIfPresent(key, value)
}

// Get retrieves a value from the underlying cache.
func (kv *KV[V]) Get(key string) (V, error) {
	return kv.data.Get(key)
}

// Del removes the key from the underlying cache and the trie index.
func (kv *KV[V]) Del(key string) error {
	kv.mux.Lock()
	defer kv.mux.Unlock()

	if err := kv.delete(key); err != nil {
		// If trie delete failed (key not found), we still try to del from data just in case,
		// essentially behaving idempotently.
	}

	return kv.data.Del(key)
}

// ListByPrefix returns all values with keys starting with the given prefix.
// Result is ordered lexicographically by key.
func (kv *KV[V]) ListByPrefix(prefix string) ([]V, error) {
	kv.mux.RLock()
	defer kv.mux.RUnlock()

	// 1. Navigate to the node covering the prefix.
	node := kv.trie
	searchKey := []byte(prefix)

	// Tracks the path string constructed so far during descent
	var path []byte

	for len(searchKey) > 0 {
		// Find child starting with the next byte
		idx, found := node.findChild(searchKey[0])
		if !found {
			return nil, nil // Prefix not found
		}

		child := node.children[idx]

		// Check how much of the child's segment matches the search key
		common := commonPrefixLen(child.b, searchKey)

		// Append current segment to path for future lookup
		path = append(path, child.b[:common]...)

		if common < len(searchKey) {
			// We haven't consumed the full search key yet.
			if common < len(child.b) {
				// The search key diverges from the existing path in the middle of a node.
				// e.g. Node has "apple", we search for "apply".
				// Common is "appl", but 'e' != 'y'. No match.
				return nil, nil
			}
			// Consumed this entire node, move to next level
			searchKey = searchKey[common:]
			node = child
		} else {
			// We matched the entire search key!
			// The rest of this node (if any) and all its children are matches.
			// We must reconstruct the FULL path to this node for the DFS.

			// If the child segment was longer than the remaining search key,
			// we need to add the *rest* of the child segment to the path
			// before starting DFS, because DFS assumes it starts *at* the node
			// passed to it.
			remainingNodeSegment := child.b[common:]

			startNode := child
			startPath := append(path, remainingNodeSegment...)

			return kv.dfs(startNode, startPath)
		}
	}

	// If we are here, the prefix was empty, so we list everything from root.
	return kv.dfs(kv.trie, []byte{})
}

// Snapshot returns a copy of the underlying cache.
func (kv *KV[V]) Snapshot() map[string]V {
	return kv.data.Snapshot()
}

// Len returns the size of the underlying cache.
func (kv *KV[V]) Len() int {
	return kv.data.Len()
}

// --- Internal Trie Helpers ---

func (kv *KV[V]) insert(key string) {
	node := kv.trie
	keyBytes := []byte(key)

	// Empty key is stored at the root
	if len(keyBytes) == 0 {
		node.terminal = true
		return
	}

	for len(keyBytes) > 0 {
		idx, found := node.findChild(keyBytes[0])

		if !found {
			// No matching child, insert a new leaf node for the rest of the key
			newNode := &trieNode{
				b:        keyBytes,
				terminal: true,
			}
			node.addChildAt(newNode, idx)
			return
		}

		child := node.children[idx]
		common := commonPrefixLen(child.b, keyBytes)

		// Case 1: The child matches the key prefix entirely.
		// e.g. child="test", key="tester" (common=4)
		if common == len(child.b) {
			keyBytes = keyBytes[common:]
			node = child
			// If key is exhausted, mark this existing node as terminal
			if len(keyBytes) == 0 {
				node.terminal = true
			}
			continue
		}

		// Case 2: Partial match. We need to split the child node.
		// e.g. child="testing", key="tester" (common=4 "test")

		// 1. Shrink the child to the common prefix
		// We create a new node 'branch' that represents the common part.
		// Actually, we can reuse the 'child' struct as the branch to keep pointers valid,
		// and move its original distinct suffix to a new child.

		origSuffix := child.b[common:]
		newSuffix := keyBytes[common:]

		// Create a node representing the rest of the original child
		restNode := &trieNode{
			b:        origSuffix,
			children: child.children, // Inherit children
			terminal: child.terminal, // Inherit terminal status
		}

		// Reset current child to be the common prefix branch
		child.b = child.b[:common]
		child.children = []*trieNode{} // Clear children, we will add restNode back
		child.terminal = false         // It's a branch now (unless new key ends here)

		// Add the rest of the original node as a child
		child.addChild(restNode)

		// Now handle the new key part
		if len(newSuffix) == 0 {
			// The new key ended exactly at the split point
			child.terminal = true
		} else {
			// The new key continues
			newNode := &trieNode{
				b:        newSuffix,
				terminal: true,
			}
			child.addChild(newNode)
		}
		return
	}
}

func (kv *KV[V]) delete(key string) error {
	// Recursive deletion to handle cleanup of empty nodes on the way up.
	// Returns true if the child should be removed from the parent's list.
	var del func(n *trieNode, k []byte) bool
	del = func(n *trieNode, k []byte) bool {
		if len(k) == 0 {
			if n.terminal {
				n.terminal = false
				// If leaf (no children), prune it.
				return len(n.children) == 0
			}
			return false // Key not found or already deleted
		}

		idx, found := n.findChild(k[0])
		if !found {
			return false // Key not found
		}

		child := n.children[idx]
		common := commonPrefixLen(child.b, k)

		// If path doesn't match fully, key isn't here
		if common != len(child.b) {
			return false
		}

		// Recursively delete from child
		shouldRemove := del(child, k[common:])

		if shouldRemove {
			// Remove child from slice
			copy(n.children[idx:], n.children[idx+1:])
			n.children[len(n.children)-1] = nil // avoid memory leak
			n.children = n.children[:len(n.children)-1]

			// If n is not terminal and has no children, it can be pruned too
			return !n.terminal && len(n.children) == 0
		}

		return false
	}

	// We don't delete the root, just its children
	del(kv.trie, []byte(key))
	return nil
}

func (kv *KV[V]) dfs(node *trieNode, currentPath []byte) ([]V, error) {
	var res []V

	if node.terminal {
		// Key reconstruction complete, fetch from data
		val, err := kv.data.Get(string(currentPath))
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}

	for _, child := range node.children {
		// Construct path for child
		// Note: append creates a new slice, which is safer for recursion than sharing a buffer
		// though slightly more allocation heavy. Given maxKeyLength constraint is theoretical,
		// this is robust.
		childPath := append(currentPath, child.b...)
		childRes, err := kv.dfs(child, childPath)
		if err != nil {
			return nil, err
		}
		res = append(res, childRes...)
	}
	return res, nil
}

// findChild performs a binary search to find the child index.
func (n *trieNode) findChild(c byte) (int, bool) {
	idx := sort.Search(len(n.children), func(i int) bool {
		return n.children[i].b[0] >= c
	})
	if idx < len(n.children) && n.children[idx].b[0] == c {
		return idx, true
	}
	return idx, false
}

// addChild adds a child in sorted order.
func (n *trieNode) addChild(child *trieNode) {
	idx, found := n.findChild(child.b[0])
	if found {
		// Should not happen in this logic unless overwriting,
		// but if so, replace.
		n.children[idx] = child
		return
	}
	n.addChildAt(child, idx)
}

// addChildAt inserts a child at a specific index to maintain order.
func (n *trieNode) addChildAt(child *trieNode, idx int) {
	n.children = append(n.children, nil)
	copy(n.children[idx+1:], n.children[idx:])
	n.children[idx] = child
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
