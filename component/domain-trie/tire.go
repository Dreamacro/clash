package trie

import (
	"errors"
	"strings"
)

const (
	wildcard   = "*"
	domainStep = "."
)

var (
	// ErrInvalidDomain means insert domain is invalid
	ErrInvalidDomain = errors.New("invalid domain")
)

// Trie contains the main logic for adding and searching nodes for domain segments.
// support wildcard domain (e.g *.google.com)
type Trie struct {
	root *Node
}

func isValidDomain(domain string) bool {
	return domain != "" && domain[0] != '.' && domain[len(domain)-1] != '.'
}

// Insert adds a node to the trie.
// Support
// 1. www.example.com
// 2. *.example.com
// 3. subdomain.*.example.com
func (t *Trie) Insert(domain string, data interface{}) error {
	if !isValidDomain(domain) {
		return ErrInvalidDomain
	}

	parts := strings.Split(domain, domainStep)
	node := t.root
	// reverse storage domain part to save space
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if !node.hasChild(part) {
			node.addChild(part, newNode(nil))
		}

		node = node.getChild(part)
	}

	node.Data = data
	return nil
}

// Search is the most important part of the Trie.
// Priority as:
// 1. static part
// 2. wildcard domain
func (t *Trie) Search(domain string) *Node {
	if !isValidDomain(domain) {
		return nil
	}
	parts := strings.Split(domain, domainStep)

	n := t.root
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		var child *Node
		if !n.hasChild(part) {
			if !n.hasChild(wildcard) {
				return nil
			}

			child = n.getChild(wildcard)
		} else {
			child = n.getChild(part)
		}

		n = child
	}

	if n.Data == nil {
		return nil
	}

	return n
}

// New returns a new, empty Trie.
func New() *Trie {
	return &Trie{root: newNode(nil)}
}
