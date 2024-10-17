package geche

// This file contains several simple map cache implementations for benchmark purposes.
// 1) Non generic version with hardcoded types.
// 2) Non thread-safe non generic version with hardcoded types.
// 3) interface{} based version.

import (
	"sync"
)

type stringCache struct {
	data map[string]string
	mux  sync.RWMutex
}

func newStringCache() *stringCache {
	return &stringCache{
		data: make(map[string]string),
	}
}

func (s *stringCache) Set(key, value string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.data[key] = value
}

func (s *stringCache) SetIfPresent(key, value string) (string, bool) {
	s.mux.Lock()
	defer s.mux.Unlock()

	old, ok := s.data[key]
	if !ok {
		return "", false
	}

	s.data[key] = value
	return old, true
}

func (s *stringCache) Get(key string) (string, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	v, ok := s.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (s *stringCache) Del(key string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	delete(s.data, key)

	return nil
}

func (s *stringCache) Snapshot() map[string]string { return nil }

func (s *stringCache) Len() int {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return len(s.data)
}

type unsafeCache struct {
	data map[string]string
}

func newUnsafeCache() *unsafeCache {
	return &unsafeCache{
		data: make(map[string]string),
	}
}

func (u *unsafeCache) Set(key, value string) {
	u.data[key] = value
}

func (u *unsafeCache) SetIfPresent(key, value string) (string, bool) {
	old, err := u.Get(key)
	if err != nil {
		return "", false
	}

	u.Set(key, value)
	return old, true
}

func (u *unsafeCache) Get(key string) (string, error) {
	v, ok := u.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (u *unsafeCache) Del(key string) error {
	delete(u.data, key)

	return nil
}

func (u *unsafeCache) Snapshot() map[string]string { return nil }

func (u *unsafeCache) Len() int {
	return len(u.data)
}

type anyCache struct {
	data map[string]any
	mux  sync.RWMutex
}

func newAnyCache() *anyCache {
	return &anyCache{
		data: make(map[string]any),
	}
}

func (a *anyCache) Set(key string, value any) {
	a.mux.Lock()
	defer a.mux.Unlock()

	a.data[key] = value
}

func (a *anyCache) Get(key string) (any, error) {
	a.mux.RLock()
	defer a.mux.RUnlock()

	v, ok := a.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (a *anyCache) Del(key string) error {
	delete(a.data, key)

	return nil
}

func (a *anyCache) Snapshot() map[string]any { return nil }

func (a *anyCache) Len() int {
	a.mux.RLock()
	defer a.mux.RUnlock()

	return len(a.data)
}
