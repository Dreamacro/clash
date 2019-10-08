package fakeip

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool_Basic(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("192.168.0.1/29")
	pool, _ := New(ipnet, 10)

	first := pool.Lookup("foo.com")
	last := pool.Lookup("bar.com")
	bar, exist := pool.LookBack(last)

	assert.True(t, first.Equal(net.IP{192, 168, 0, 2}))
	assert.True(t, last.Equal(net.IP{192, 168, 0, 3}))
	assert.True(t, exist)
	assert.Equal(t, bar, "bar.com")
}

func TestPool_Cycle(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("192.168.0.1/30")
	pool, _ := New(ipnet, 10)

	first := pool.Lookup("foo.com")
	same := pool.Lookup("baz.com")

	assert.True(t, first.Equal(same))
}

func TestPool_MaxCacheSize(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("192.168.0.1/24")
	pool, _ := New(ipnet, 2)

	first := pool.Lookup("foo.com")
	pool.Lookup("bar.com")
	pool.Lookup("baz.com")
	next := pool.Lookup("foo.com")

	assert.False(t, first.Equal(next))
}

func TestPool_Error(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("192.168.0.1/31")
	_, err := New(ipnet, 10)

	assert.Error(t, err)
}
