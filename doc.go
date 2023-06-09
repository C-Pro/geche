// Package geche (GEneric Cache) implements several types of caches
// using Go generics (requires go 1.18+).
package geche

import "errors"

var ErrNotFound = errors.New("not found")

// Geche interface is a common interface for all cache implementations.
type Geche[K comparable, V any] interface {
	Set(K, V)
	Get(K) (V, error)
	Del(K) error
	Snapshot() map[K]V
	Len() int
}
