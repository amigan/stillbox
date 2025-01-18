package cache

import "sync"

type Cache[K comparable, V any] interface {
	Get(K) (V, bool)
	Set(K, V)
	Delete(K)
	Clear()
}

type inMem[K comparable, V any] struct {
	sync.RWMutex
	m map[K]V
}

func New[K comparable, V any]() *inMem[K, V] {
	return &inMem[K, V]{
		m: make(map[K]V),
	}
}

func (c *inMem[K, V]) Get(key K) (V, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *inMem[K, V]) Set(key K, val V) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = val
}

func (c *inMem[K, V]) Delete(key K) {
	c.Lock()
	defer c.Unlock()
	delete(c.m, key)
}

func (c *inMem[K, V]) Clear() {
	c.Lock()
	defer c.Unlock()
	clear(c.m)
}
