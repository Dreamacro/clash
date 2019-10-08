package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var entries = []struct {
	key   string
	value string
}{
	{"1", "one"},
	{"2", "two"},
	{"3", "three"},
	{"4", "four"},
	{"5", "five"},
}

func TestLRUCache(t *testing.T) {
	c := NewLRUCache()

	for _, e := range entries {
		c.Set(e.key, e.value)
	}

	c.Delete("missing")
	_, ok := c.Get("missing")
	assert.False(t, ok)

	for _, e := range entries {
		value, ok := c.Get(e.key)
		if assert.True(t, ok) {
			assert.Equal(t, e.value, value.(string))
		}
	}

	for _, e := range entries {
		c.Delete(e.key)

		_, ok := c.Get(e.key)
		assert.False(t, ok)
	}
}

func TestLRUMaxAge(t *testing.T) {
	c := NewLRUCache(WithAge(86400))

	now := time.Now().Unix()
	expected := now + 86400

	// Add one expired entry
	c.Set("foo", "bar")
	c.lru.Back().Value.(*entry).expires = now

	// Reset
	c.Set("foo", "bar")
	e := c.lru.Back().Value.(*entry)
	assert.True(t, e.expires >= now)
	c.lru.Back().Value.(*entry).expires = now

	// Set a few and verify expiration times
	for _, s := range entries {
		c.Set(s.key, s.value)
		e := c.lru.Back().Value.(*entry)
		assert.True(t, e.expires >= expected && e.expires <= expected+10)
	}

	// Make sure we can get them all
	for _, s := range entries {
		_, ok := c.Get(s.key)
		assert.True(t, ok)
	}

	// Expire all entries
	for _, s := range entries {
		le, ok := c.cache[s.key]
		if assert.True(t, ok) {
			le.Value.(*entry).expires = now
		}
	}

	// Get one expired entry, which should clear all expired entries
	_, ok := c.Get("3")
	assert.False(t, ok)
	assert.Equal(t, c.lru.Len(), 0)
}

func TestLRUpdateOnGet(t *testing.T) {
	c := NewLRUCache(WithAge(86400), WithUpdateAgeOnGet())

	now := time.Now().Unix()
	expires := now + 86400/2

	// Add one expired entry
	c.Set("foo", "bar")
	c.lru.Back().Value.(*entry).expires = expires

	_, ok := c.Get("foo")
	assert.True(t, ok)
	assert.True(t, c.lru.Back().Value.(*entry).expires > expires)
}

func TestMaxSize(t *testing.T) {
	c := NewLRUCache(WithSize(2))
	// Add one expired entry
	c.Set("foo", "bar")
	_, ok := c.Get("foo")
	assert.True(t, ok)

	c.Set("bar", "foo")
	c.Set("baz", "foo")

	_, ok = c.Get("foo")
	assert.False(t, ok)
}
