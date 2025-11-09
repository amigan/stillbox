package cache

import (
	"sync"
	"time"
)

type Cache[K comparable, V any] interface {
	Get(K) (V, bool)
	Set(K, V)
	Delete(K)
	DeleteAndHoldLock(K, func())
	Clear()
}

type cacheItem[V any] struct {
	exp  int64
	elem V
}

type inMem[K comparable, V any] struct {
	sync.RWMutex
	expiration time.Duration
	m          map[K]cacheItem[V]
}

type cacheOpts struct {
	exp time.Duration
}

type Option func(*cacheOpts)

func WithExpiration(d time.Duration) Option {
	return func(o *cacheOpts) {
		o.exp = d
	}
}

func New[K comparable, V any](opts ...Option) *inMem[K, V] {
	co := cacheOpts{}

	for _, o := range opts {
		o(&co)
	}

	c := &inMem[K, V]{
		m:          make(map[K]cacheItem[V]),
		expiration: co.exp,
	}

	return c
}

func (c *inMem[K, V]) Get(key K) (V, bool) {
	c.RLock()
	defer c.RUnlock()

	v, found := c.m[key]
	if !found || (v.exp > 0 && time.Now().UnixNano() > v.exp) {
		return v.elem, false
	}

	return v.elem, true
}

func (c *inMem[K, V]) Set(key K, val V) {
	c.Lock()
	defer c.Unlock()

	var ci cacheItem[V]
	ci.elem = val
	if c.expiration > 0 {
		ci.exp = time.Now().Add(c.expiration).UnixNano()
	}

	c.m[key] = ci
}

func (c *inMem[K, V]) DeleteAndHoldLock(key K, f func()) {
	c.Lock()
	defer c.Unlock()

	delete(c.m, key)
	f()
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
