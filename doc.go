// Package geche (GEneric Cache) implements several types of caches
// using Go generics (requires go 1.18+).
package geche

import "errors"

var ErrNotFound = errors.New("not found")

type Geche[T any, K comparable] interface {
	Set(K, T)
	Get(K) (T, error)
	Del(K) error
}
