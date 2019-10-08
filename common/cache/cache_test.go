package cache

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache_Basic(t *testing.T) {
	interval := 200 * time.Millisecond
	ttl := 20 * time.Millisecond
	c := New(interval)
	c.Put("int", 1, ttl)
	c.Put("string", "a", ttl)

	i := c.Get("int")
	assert.Equal(t, i.(int), 1, "should recv 1")

	s := c.Get("string")
	assert.Equal(t, s.(string), "a", "should recv 'a'")
}

func TestCache_TTL(t *testing.T) {
	interval := 200 * time.Millisecond
	ttl := 20 * time.Millisecond
	now := time.Now()
	c := New(interval)
	c.Put("int", 1, ttl)
	c.Put("int2", 2, ttl)

	i := c.Get("int")
	_, expired := c.GetWithExpire("int2")
	assert.Equal(t, i.(int), 1, "should recv 1")
	assert.True(t, now.Before(expired))

	time.Sleep(ttl * 2)
	i = c.Get("int")
	j, _ := c.GetWithExpire("int2")
	assert.Nil(t, i, "should recv nil")
	assert.Nil(t, j, "should recv nil")
}

func TestCache_AutoCleanup(t *testing.T) {
	interval := 10 * time.Millisecond
	ttl := 15 * time.Millisecond
	c := New(interval)
	c.Put("int", 1, ttl)

	time.Sleep(ttl * 2)
	i := c.Get("int")
	j, _ := c.GetWithExpire("int")
	assert.Nil(t, i, "should recv nil")
	assert.Nil(t, j, "should recv nil")
}

func TestCache_AutoGC(t *testing.T) {
	sign := make(chan struct{})
	go func() {
		interval := 10 * time.Millisecond
		ttl := 15 * time.Millisecond
		c := New(interval)
		c.Put("int", 1, ttl)
		sign <- struct{}{}
	}()

	<-sign
	runtime.GC()
}
