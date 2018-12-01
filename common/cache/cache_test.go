package cache

import (
	"runtime"
	"testing"
	"time"
)

func TestCache_Basic(t *testing.T) {
	interval := 200 * time.Millisecond
	ttl := 20 * time.Millisecond
	c := New(interval)
	c.Put("int", 1, ttl)
	c.Put("string", "a", ttl)

	i := c.Get("int")
	if i.(int) != 1 {
		t.Error("should recv 1")
	}

	s := c.Get("string")
	if s.(string) != "a" {
		t.Error("should recv 'a'")
	}
}

func TestCache_TTL(t *testing.T) {
	interval := 200 * time.Millisecond
	ttl := 20 * time.Millisecond
	c := New(interval)
	c.Put("int", 1, ttl)

	i := c.Get("int")
	if i.(int) != 1 {
		t.Error("should recv 1")
	}

	time.Sleep(ttl * 2)
	i = c.Get("int")
	if i != nil {
		t.Error("should recv nil")
	}
}

func TestCache_AutoCleanup(t *testing.T) {
	interval := 10 * time.Millisecond
	ttl := 15 * time.Millisecond
	c := New(interval)
	c.Put("int", 1, ttl)

	time.Sleep(ttl * 2)
	i := c.Get("int")
	if i != nil {
		t.Error("should recv nil")
	}
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
