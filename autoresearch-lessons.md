# Autoresearch Lessons: KVCache Optimization

Through iterative optimization of the radix tree-based `KVCache`, we achieved a **47.3% performance improvement** (from `1061` ns/op down to `558.3` ns/op). Here are the key lessons learned:

### 1. Leverage Generics to Avoid Interface/Type Conversion Allocations
- **Problem**: Converting string keys to `[]byte` values for storage in the trie node segments (`[]byte`) caused heap allocations on every key insert/split.
- **Solution**: Making the internal trie node struct generic (`trieCacheNode[K byteSlice]`) allows storing path segments directly in their native key type `K` (`string` or `[]byte`). For string keys, slicing is an allocation-free header copy, resulting in zero key allocations during trie traversal and updates.
- **Result**: Reduced allocations to `0 allocs/op` (only 26 B/op instead of the baseline).

### 2. Bypass Pointer Chasing with Inline Struct Fields
- **Problem**: Scanning node children slices required dereferencing the first byte of each child's segment (i.e. accessing `node.children[i].b[0]`), which triggers pointer chasing through slice headers.
- **Solution**: Inlining the first byte directly in the node struct (`b0 byte`) allows contiguous scanning of children list without pointer chasing.
- **Result**: Reduced memory latency during search scans (improved performance from 800.2 ns/op to 593.3 ns/op).

### 3. Avoid Binary Search Overhead on Small Slices
- **Problem**: Radix tree nodes typically have a very small branching factor (small number of child nodes). Standard library `sort.Search` closure allocation and function call overhead outweighed the $O(\log N)$ algorithmic benefits.
- **Solution**: Replacing binary search with a simple linear scan over child nodes was significantly faster.
- **Result**: Substantial latency savings (improved performance from 1002 ns/op to 804.6 ns/op).

### 4. Build Slices Directly Rather than Appending Incrementally
- **Problem**: Splitting a node required creating multiple child nodes, calling helper methods (`addChild`, `addChildAt`), and resizing slices iteratively.
- **Solution**: In hot paths (like splitting a node in `insert`), pre-building the child array with the exact capacity and element count (e.g. 2-element array for the split parts) avoids iterative slice resizing and helper call overhead.
