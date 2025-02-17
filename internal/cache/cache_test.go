package cache_test

import (
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/cache"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	c := cache.New[int, string]()
	c.Set(4, "asd")
	g, ok := c.Get(4)
	assert.Equal(t, "asd", g)
	assert.True(t, ok)

	_, ok = c.Get(8)
	assert.False(t, ok)

	c.Set(7, "fg")

	c.Delete(4)

	g, ok = c.Get(4)
	assert.False(t, ok)
	assert.NotEqual(t, "asd", g)

	c.Clear()
	g, ok = c.Get(7)
	assert.False(t, ok)
	assert.NotEqual(t, "fg", g)
}

func TestCacheTime(t *testing.T) {
	c := cache.New[int, string](cache.WithExpiration(100 * time.Millisecond))
	c.Set(4, "asd")
	time.Sleep(20 * time.Millisecond)
	g, ok := c.Get(4)
	assert.Equal(t, "asd", g)
	assert.True(t, ok)

	c.Set(2, "gff")
	time.Sleep(120 * time.Millisecond)
	g, ok = c.Get(2)
	assert.False(t, ok)

	_, ok = c.Get(8)
	assert.False(t, ok)

	c.Set(7, "fg")

	c.Delete(4)

	g, ok = c.Get(4)
	assert.False(t, ok)
	assert.NotEqual(t, "asd", g)

	c.Clear()
	g, ok = c.Get(7)
	assert.False(t, ok)
	assert.NotEqual(t, "fg", g)
}
