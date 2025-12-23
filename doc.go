// Package geche (GEneric Cache) implements several types of caches
// using Go generics (requires go 1.18+).
package geche

import "errors"

var ErrNotFound = errors.New("not found")

// Geche interface is a common interface for all cache implementations.
type Geche[K comparable, V any] interface {
	Set(K, V)
	// SetIfPresent sets the kv only if the key was already present, and returns the previous value (if any) and whether the insertion was performed
	SetIfPresent(K, V) (V, bool)
	// SetIfAbsent sets the kv only if the key didn't exist yet, and returns the existing value (if any) and whether the insertion was performed
	SetIfAbsent(K, V) (V, bool)
	Get(K) (V, error)
	Del(K) error
	Snapshot() map[K]V
	Len() int
}
