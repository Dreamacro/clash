package trie

import (
	"net"
	"testing"
)

var localIP = net.IP{127, 0, 0, 1}

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

	if !node.Data.(net.IP).Equal(localIP) {
		t.Error("should equal 127.0.0.1")
	}

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
