package cache

// Modified by https://github.com/die-net/lrucache

import (
	"container/list"
	"sync"
	"time"
)

// Option is part of Functional Options Pattern
type Option func(*LruCache)

// EvictCallback is used to get a callback when a cache entry is evicted
type EvictCallback func(key interface{}, value interface{})

// WithEvict set the evict callback
func WithEvict(cb EvictCallback) Option {
	return func(l *LruCache) {
		l.onEvict = cb
	}
}

// WithUpdateAgeOnGet update expires when Get element
func WithUpdateAgeOnGet() Option {
	return func(l *LruCache) {
		l.updateAgeOnGet = true
	}
}

// WithAge defined element max age (second)
func WithAge(maxAge int64) Option {
	return func(l *LruCache) {
		l.maxAge = maxAge
	}
}

// WithSize defined max length of LruCache
func WithSize(maxSize int) Option {
	return func(l *LruCache) {
		l.maxSize = maxSize
	}
}

// LruCache is a thread-safe, in-memory lru-cache that evicts the
// least recently used entries from memory when (if set) the entries are
// older than maxAge (in seconds).  Use the New constructor to create one.
type LruCache struct {
	maxAge         int64
	maxSize        int
	mu             sync.Mutex
	cache          map[interface{}]*list.Element
	lru            *list.List // Front is least-recent
	updateAgeOnGet bool
	onEvict        EvictCallback
}

// NewLRUCache creates an LruCache
func NewLRUCache(options ...Option) *LruCache {
	lc := &LruCache{
		lru:   list.New(),
		cache: make(map[interface{}]*list.Element),
	}

	for _, option := range options {
		option(lc)
	}

	return lc
}

// Get returns the interface{} representation of a cached response and a bool
// set to true if the key was found.
func (c *LruCache) Get(key interface{}) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	le, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	if c.maxAge > 0 && le.Value.(*entry).expires <= time.Now().Unix() {
		c.deleteElement(le)
		c.maybeDeleteOldest()

		return nil, false
	}

	c.lru.MoveToBack(le)
	entry := le.Value.(*entry)
	if c.maxAge > 0 && c.updateAgeOnGet {
		entry.expires = time.Now().Unix() + c.maxAge
	}
	value := entry.value

	return value, true
}

// Exist returns if key exist in cache but not put item to the head of linked list
func (c *LruCache) Exist(key interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.cache[key]
	return ok
}

// Set stores the interface{} representation of a response for a given key.
func (c *LruCache) Set(key interface{}, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expires := int64(0)
	if c.maxAge > 0 {
		expires = time.Now().Unix() + c.maxAge
	}

	if le, ok := c.cache[key]; ok {
		c.lru.MoveToBack(le)
		e := le.Value.(*entry)
		e.value = value
		e.expires = expires
	} else {
		e := &entry{key: key, value: value, expires: expires}
		c.cache[key] = c.lru.PushBack(e)

		if c.maxSize > 0 {
			if len := c.lru.Len(); len > c.maxSize {
				c.deleteElement(c.lru.Front())
			}
		}
	}

	c.maybeDeleteOldest()
}

// Delete removes the value associated with a key.
func (c *LruCache) Delete(key string) {
	c.mu.Lock()

	if le, ok := c.cache[key]; ok {
		c.deleteElement(le)
	}

	c.mu.Unlock()
}

func (c *LruCache) maybeDeleteOldest() {
	if c.maxAge > 0 {
		now := time.Now().Unix()
		for le := c.lru.Front(); le != nil && le.Value.(*entry).expires <= now; le = c.lru.Front() {
			c.deleteElement(le)
		}
	}
}

func (c *LruCache) deleteElement(le *list.Element) {
	c.lru.Remove(le)
	e := le.Value.(*entry)
	delete(c.cache, e.key)
	if c.onEvict != nil {
		c.onEvict(e.key, e.value)
	}
}

// Clear delete all the items
func (c *LruCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[interface{}]*list.Element)
}

type entry struct {
	key     interface{}
	value   interface{}
	expires int64
}
