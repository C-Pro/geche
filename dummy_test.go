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

func (c *stringCache) Set(key, value string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.data[key] = value
}

func (c *stringCache) Get(key string) (string, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	v, ok := c.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (c *stringCache) Del(key string) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.data, key)

	return nil
}

type unsafeCache struct {
	data map[string]string
}

func newUnsafeCache() *unsafeCache {
	return &unsafeCache{
		data: make(map[string]string),
	}
}

func (c *unsafeCache) Set(key, value string) {
	c.data[key] = value
}

func (c *unsafeCache) Get(key string) (string, error) {
	v, ok := c.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (c *unsafeCache) Del(key string) error {
	delete(c.data, key)

	return nil
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

func (c *anyCache) Set(key string, value any) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.data[key] = value
}

func (c *anyCache) Get(key string) (any, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	v, ok := c.data[key]
	if !ok {
		return v, ErrNotFound
	}

	return v, nil
}

func (c *anyCache) Del(key string) error {
	delete(c.data, key)

	return nil
}
