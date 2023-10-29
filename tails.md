# Tail aggregation storage optimization

Instead of storing each character of the trie in the separate node, we can store a "tail" string in the stingle node. This will reduce the number of nodes in the trie and will reduce the memory consumption.

"Tail" is the suffix part of the key that belongs to one key only.

For example before optimiztion keys "apple" and "approve" were stored this way (one node per character):

a - p - p - l - e
        |
        r - o - v - e

After "tail aggregation" the suffix parts of both keys can be aggregated in the single node:

a - p - p - le
        |
        rove

In the first case we have 9 nodes, in the second case we have 5 nodes. Since the node is a rather large structure with several pointers, reducing number of the nodes can significantly reduce memory consumption.


## Implementation

### Adding the new key

When adding the new key we should do the same loop over key characters finding corresponding nodes in the existing trie. There are two cases:

1. We reach the end of the key string.
2. We reach the key character that does not have corresponding trie node.

In first case we keep the current behaviour: just setting "terminal" flag in the last node of the key.
In the second case instead of adding the "tail" of the key as new nodes one character at a time we add the "tail" as a single node containing the whole tail string insteead of a single character, and setting this node's terminal flag to true.

Oopps. Now we have a third case: while looking for the next character of the key we can reach the node that already has the tail string.
In this case the easiest option is to delete tail node (but remember the tail string first) and start moving on both strings (key and tail) and adding new nodes until we reach end of one or both strings.

If we reached the end of both strings at the same time this means it is the same key, we just need to update the value. We can optimize this case by comparing tail and remainder of the key. If they are equal, no need to split nodes - just update the value.

If one of the strings is longer than the other, once we reach the shorter string's key we set the terminal flag and new node with the rest of the longer string as a tail.

### Deleting the key

When deleting the key first we do loop over key characters finding corresponding nodes in the existing trie until we reach the end of the key. It should be a terminal node.

If this node has descendants, we just set the terminal flag to false and return.

If it does not have descendants, we need to delete the node and all its ancestors that do not have descendants. We can do it by traversing the trie from the end of the key to the root and deleting all nodes that do not have descendants. The logic here does not change much compared with the case when nodes are single-character. "Tail" node containing string can only be in the end by definition, and in this implementation it can't have descendants.

### Listing by prefix

Listing by prefix should not change much. If we encounter tail node after we exhausted prefix string - we emit this key's value as a part of the result stet, and move along.

If we encounter tail node before we exhausted prefix string - we need to compare prefix string with the tail string. If remaining part of the prefix is the tail's prefix, we add this key's value to the result set and move along. If it is not, just continue our loop as usual.
