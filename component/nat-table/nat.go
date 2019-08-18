package nat

import (
	"net"
	"sync"
)

// Packet NAT table
type Table struct {
	mu      sync.RWMutex
	mapping map[string]*element
}

type element struct {
	pc   net.PacketConn
	addr net.Addr
}

func NewTable() *Table {
	return &Table{
		mapping: make(map[string]*element),
	}
}

func (t *Table) Get(key string) (pc net.PacketConn, addr net.Addr) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ele, ok := t.mapping[key]
	if !ok {
		return
	}
	return ele.pc, ele.addr
}

func (t *Table) Add(key string, pc net.PacketConn, addr net.Addr, f func()) {
	t.Set(key, pc, addr)

	go func() {
		f()
		if pc, _ := t.Del(key); pc != nil {
			pc.Close()
		}
	}()
}

func (t *Table) Set(key string, pc net.PacketConn, addr net.Addr) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.mapping[key] = &element{
		pc:   pc,
		addr: addr,
	}
}

func (t *Table) Del(key string) (pc net.PacketConn, addr net.Addr) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if ele, ok := t.mapping[key]; ok {
		delete(t.mapping, key)
		return ele.pc, ele.addr
	}
	return
}
