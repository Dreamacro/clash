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
type EvictCallback = func(key interface{}, value interface{})

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

// WithStale decide whether Stale return is enabled.
// If this feature is enabled, element will not get Evicted according to `WithAge`.
func WithStale(stale bool) Option {
	return func(l *LruCache) {
		l.staleReturn = stale
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
	staleReturn    bool
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
	entry := c.get(key)
	if entry == nil {
		return nil, false
	}
	value := entry.value

	return value, true
}

// GetWithExpire returns the interface{} representation of a cached response,
// a time.Time Give expected expires,
// and a bool set to true if the key was found.
// This method will NOT check the maxAge of element and will NOT update the expires.
func (c *LruCache) GetWithExpire(key interface{}) (interface{}, time.Time, bool) {
	entry := c.get(key)
	if entry == nil {
		return nil, time.Time{}, false
	}

	return entry.value, time.Unix(entry.expires, 0), true
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
	expires := int64(0)
	if c.maxAge > 0 {
		expires = time.Now().Unix() + c.maxAge
	}
	c.SetWithExpire(key, value, time.Unix(expires, 0))
}

// SetWithExpire stores the interface{} representation of a response for a given key and given expires.
// The expires time will round to second.
func (c *LruCache) SetWithExpire(key interface{}, value interface{}, expires time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if le, ok := c.cache[key]; ok {
		c.lru.MoveToBack(le)
		e := le.Value.(*entry)
		e.value = value
		e.expires = expires.Unix()
	} else {
		e := &entry{key: key, value: value, expires: expires.Unix()}
		c.cache[key] = c.lru.PushBack(e)

		if c.maxSize > 0 {
			if len := c.lru.Len(); len > c.maxSize {
				c.deleteElement(c.lru.Front())
			}
		}
	}

	c.maybeDeleteOldest()
}

// CloneTo clone and overwrite elements to another LruCache
func (c *LruCache) CloneTo(n *LruCache) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	n.lru = list.New()
	n.cache = make(map[interface{}]*list.Element)

	for e := c.lru.Front(); e != nil; e = e.Next() {
		elm := e.Value.(*entry)
		n.cache[elm.key] = n.lru.PushBack(elm)
	}
}

func (c *LruCache) get(key interface{}) *entry {
	c.mu.Lock()
	defer c.mu.Unlock()

	le, ok := c.cache[key]
	if !ok {
		return nil
	}

	if !c.staleReturn && c.maxAge > 0 && le.Value.(*entry).expires <= time.Now().Unix() {
		c.deleteElement(le)
		c.maybeDeleteOldest()

		return nil
	}

	c.lru.MoveToBack(le)
	entry := le.Value.(*entry)
	if c.maxAge > 0 && c.updateAgeOnGet {
		entry.expires = time.Now().Unix() + c.maxAge
	}
	return entry
}

// Delete removes the value associated with a key.
func (c *LruCache) Delete(key interface{}) {
	c.mu.Lock()

	if le, ok := c.cache[key]; ok {
		c.deleteElement(le)
	}

	c.mu.Unlock()
}

func (c *LruCache) maybeDeleteOldest() {
	if !c.staleReturn && c.maxAge > 0 {
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

type entry struct {
	key     interface{}
	value   interface{}
	expires int64
}
