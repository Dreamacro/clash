package nat

import (
	"net"
	"sync"
)

type Table struct {
	mapping sync.Map
}

type element struct {
	RemoteAddr net.Addr
	RemoteConn net.PacketConn
}

func (t *Table) Set(key string, pc net.PacketConn, addr net.Addr) {
	// set conn read timeout
	t.mapping.Store(key, &element{
		RemoteConn: pc,
		RemoteAddr: addr,
	})
}

func (t *Table) Get(key string) (net.PacketConn, net.Addr) {
	item, exist := t.mapping.Load(key)
	if !exist {
		return nil, nil
	}
	elm := item.(*element)
	return elm.RemoteConn, elm.RemoteAddr
}

func (t *Table) GetOrCreateLock(key string) (*sync.WaitGroup, bool) {
	item, loaded := t.mapping.LoadOrStore(key, &sync.WaitGroup{})
	return item.(*sync.WaitGroup), loaded
}

func (t *Table) Delete(key string) {
	t.mapping.Delete(key)
}

// New return *Cache
func New() *Table {
	return &Table{}
}
