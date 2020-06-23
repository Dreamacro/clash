package trie

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

var localIP = net.IP{127, 0, 0, 1}

func TestTrie_Basic(t *testing.T) {
	tree := New()
	domains := []string{
		"example.com",
		"google.com",
		"localhost",
	}

	for _, domain := range domains {
		tree.Insert(domain, localIP)
	}

	node := tree.Search("example.com")
	assert.NotNil(t, node)
	assert.True(t, node.Data.(net.IP).Equal(localIP))
	assert.NotNil(t, tree.Insert("", localIP))
	assert.Nil(t, tree.Search(""))
	assert.NotNil(t, tree.Search("localhost"))
}

func TestTrie_Wildcard(t *testing.T) {
	tree := New()
	domains := []string{
		"*.example.com",
		"sub.*.example.com",
		"*.dev",
		".org",
		".example.net",
		".apple.*",
		"+.foo.com",
		"+.stun.*.*",
		"+.stun.*.*.*",
		"+.stun.*.*.*.*",
		"stun.l.google.com",
	}

	for _, domain := range domains {
		tree.Insert(domain, localIP)
	}

	assert.NotNil(t, tree.Search("sub.example.com"))
	assert.NotNil(t, tree.Search("sub.foo.example.com"))
	assert.NotNil(t, tree.Search("test.org"))
	assert.NotNil(t, tree.Search("test.example.net"))
	assert.NotNil(t, tree.Search("test.apple.com"))
	assert.NotNil(t, tree.Search("test.foo.com"))
	assert.NotNil(t, tree.Search("foo.com"))
	assert.NotNil(t, tree.Search("global.stun.website.com"))
	assert.Nil(t, tree.Search("foo.sub.example.com"))
	assert.Nil(t, tree.Search("foo.example.dev"))
	assert.Nil(t, tree.Search("example.com"))
}

func TestTrie_Priority(t *testing.T) {
	tree := New()
	domains := []string{
		".dev",
		"example.dev",
		"*.example.dev",
		"test.example.dev",
	}

	assertFn := func(domain string, data int) {
		node := tree.Search(domain)
		assert.NotNil(t, node)
		assert.Equal(t, data, node.Data)
	}

	for idx, domain := range domains {
		tree.Insert(domain, idx)
	}

	assertFn("test.dev", 0)
	assertFn("foo.bar.dev", 0)
	assertFn("example.dev", 1)
	assertFn("foo.example.dev", 2)
	assertFn("test.example.dev", 3)
}

func TestTrie_Boundary(t *testing.T) {
	tree := New()
	tree.Insert("*.dev", localIP)

	assert.NotNil(t, tree.Insert(".", localIP))
	assert.NotNil(t, tree.Insert("..dev", localIP))
	assert.Nil(t, tree.Search("dev"))
}
