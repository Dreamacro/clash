package trie

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

var localIP = []net.IP{{127, 0, 0, 1}, {127, 0, 0, 2}}

func TestTrie_Basic(t *testing.T) {
	tree := New()
	domains := []string{
		"example.com",
		"google.com",
	}

	for _, domain := range domains {
		tree.Insert(domain, localIP)
	}

	node := tree.Search("example.com")
	if node == nil {
		t.Error("should not recv nil")
	}

	assert.Equal(t, localIP, node.Data.([]net.IP), "should be the same IP addresses")

	if tree.Insert("", localIP) == nil {
		t.Error("should return error")
	}
}

func TestTrie_Wildcard(t *testing.T) {
	tree := New()
	domains := []string{
		"*.example.com",
		"sub.*.example.com",
		"*.dev",
	}

	for _, domain := range domains {
		tree.Insert(domain, localIP)
	}

	if tree.Search("sub.example.com") == nil {
		t.Error("should not recv nil")
	}

	if tree.Search("sub.foo.example.com") == nil {
		t.Error("should not recv nil")
	}

	if tree.Search("foo.sub.example.com") != nil {
		t.Error("should recv nil")
	}

	if tree.Search("foo.example.dev") != nil {
		t.Error("should recv nil")
	}

	if tree.Search("example.com") != nil {
		t.Error("should recv nil")
	}
}

func TestTrie_Boundary(t *testing.T) {
	tree := New()
	tree.Insert("*.dev", localIP)

	if err := tree.Insert(".", localIP); err == nil {
		t.Error("should recv err")
	}

	if err := tree.Insert(".com", localIP); err == nil {
		t.Error("should recv err")
	}

	if tree.Search("dev") != nil {
		t.Error("should recv nil")
	}

	if tree.Search(".dev") != nil {
		t.Error("should recv nil")
	}
}
