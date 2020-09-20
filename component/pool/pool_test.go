package pool

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func lg() Factory {
	initial := -1
	return func(context.Context) interface{} {
		initial++
		return initial
	}
}

func TestPool_Basic(t *testing.T) {
	g := lg()
	pool := New(g)

	elm := pool.Get()
	assert.Equal(t, 0, elm.(int))
	pool.Put(elm)
	assert.Equal(t, 0, pool.Get().(int))
	assert.Equal(t, 1, pool.Get().(int))
}

func TestPool_MaxSize(t *testing.T) {
	g := lg()
	size := 5
	pool := New(g, WithSize(size))

	items := []interface{}{}

	for i := 0; i < size; i++ {
		items = append(items, pool.Get())
	}

	extra := pool.Get()
	assert.Equal(t, size, extra.(int))

	for _, item := range items {
		pool.Put(item)
	}

	pool.Put(extra)

	for _, item := range items {
		assert.Equal(t, item.(int), pool.Get().(int))
	}
}

func TestPool_MaxAge(t *testing.T) {
	g := lg()
	pool := New(g, WithAge(20))

	pool.Put(pool.Get())

	elm := pool.Get()
	assert.Equal(t, 0, elm.(int))
	pool.Put(elm)

	time.Sleep(time.Millisecond * 22)
	assert.Equal(t, 1, pool.Get().(int))
}

func TestPool_AutoGC(t *testing.T) {
	g := lg()

	sign := make(chan int)
	pool := New(g, WithEvict(func(item interface{}) {
		sign <- item.(int)
	}))

	assert.Equal(t, 0, pool.Get().(int))
	pool.Put(2)

	runtime.GC()

	select {
	case num := <-sign:
		assert.Equal(t, 2, num)
	case <-time.After(time.Second):
		assert.Fail(t, "something wrong")
	}
}
