package pool

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllocGet(t *testing.T) {
	alloc := NewAllocator()
	assert.Nil(t, alloc.Get(0))
	assert.Equal(t, 1, len(alloc.Get(1)))
	assert.Equal(t, 2, len(alloc.Get(2)))
	assert.Equal(t, 3, len(alloc.Get(3)))
	assert.Equal(t, 4, cap(alloc.Get(3)))
	assert.Equal(t, 4, cap(alloc.Get(4)))
	assert.Equal(t, 1023, len(alloc.Get(1023)))
	assert.Equal(t, 1024, cap(alloc.Get(1023)))
	assert.Equal(t, 1024, len(alloc.Get(1024)))
	assert.Equal(t, 65536, len(alloc.Get(65536)))
	assert.Nil(t, alloc.Get(65537))
}

func TestAllocPut(t *testing.T) {
	alloc := NewAllocator()
	assert.NotNil(t, alloc.Put(nil), "put nil misbehavior")
	assert.NotNil(t, alloc.Put(make([]byte, 3, 3)), "put elem:3 []bytes misbehavior")
	assert.Nil(t, alloc.Put(make([]byte, 4, 4)), "put elem:4 []bytes misbehavior")
	assert.Nil(t, alloc.Put(make([]byte, 1023, 1024)), "put elem:1024 []bytes misbehavior")
	assert.Nil(t, alloc.Put(make([]byte, 65536, 65536)), "put elem:65536 []bytes misbehavior")
	assert.NotNil(t, alloc.Put(make([]byte, 65537, 65537)), "put elem:65537 []bytes misbehavior")
}

func TestAllocPutThenGet(t *testing.T) {
	alloc := NewAllocator()
	data := alloc.Get(4)
	alloc.Put(data)
	newData := alloc.Get(4)

	assert.Equal(t, cap(data), cap(newData), "different cap while alloc.Get()")
}

func BenchmarkMSB(b *testing.B) {
	for i := 0; i < b.N; i++ {
		msb(rand.Int())
	}
}
