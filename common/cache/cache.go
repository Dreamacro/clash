package cache

import (
	"runtime"
	"sync"
	"time"
)

// Cache store element with a expired time
type Cache struct {
	*cache
}

type cache struct {
	mapping sync.Map
	janitor *janitor
}

type element struct {
	Expired time.Time
	Payload interface{}
}

// Put element in Cache with its ttl
func (c *cache) Put(key interface{}, payload interface{}, ttl time.Duration) {
	c.mapping.Store(key, &element{
		Payload: payload,
		Expired: time.Now().Add(ttl),
	})
}

// Get element in Cache, and drop when it expired
func (c *cache) Get(key interface{}) interface{} {
	item, exist := c.mapping.Load(key)
	if !exist {
		return nil
	}
	elm := item.(*element)
	// expired
	if time.Since(elm.Expired) > 0 {
		c.mapping.Delete(key)
		return nil
	}
	return elm.Payload
}

// GetWithExpire element in Cache with Expire Time
func (c *cache) GetWithExpire(key interface{}) (payload interface{}, expired time.Time) {
	item, exist := c.mapping.Load(key)
	if !exist {
		return
	}
	elm := item.(*element)
	// expired
	if time.Since(elm.Expired) > 0 {
		c.mapping.Delete(key)
		return
	}
	return elm.Payload, elm.Expired
}

func (c *cache) cleanup() {
	c.mapping.Range(func(k, v interface{}) bool {
		key := k.(string)
		elm := v.(*element)
		if time.Since(elm.Expired) > 0 {
			c.mapping.Delete(key)
		}
		return true
	})
}

type janitor struct {
	interval time.Duration
	stop     chan struct{}
}

func (j *janitor) process(c *cache) {
	ticker := time.NewTicker(j.interval)
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c *Cache) {
	c.janitor.stop <- struct{}{}
}

// New return *Cache
func New(interval time.Duration) *Cache {
	j := &janitor{
		interval: interval,
		stop:     make(chan struct{}),
	}
	c := &cache{janitor: j}
	go j.process(c)
	C := &Cache{c}
	runtime.SetFinalizer(C, stopJanitor)
	return C
}
