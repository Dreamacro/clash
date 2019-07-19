package adapters

import (
	"net"
	"sync"
	"time"

	"github.com/Dreamacro/clash/common/cache"
)

var (
	natMap *NATMap
	once   sync.Once

	natTimeout = 120 * time.Second
)

type NATMap struct {
	cache   *cache.Cache
	Timeout time.Duration
}

func (m *NATMap) Get(key string) net.Conn {
	item := m.cache.Get(key)
	if item == nil {
		return nil
	}
	return item.(net.Conn)
}

func (m *NATMap) Set(key string, conn net.Conn) {
	m.cache.Put(key, conn, m.Timeout)
}

func newNATMap() *NATMap {
	return &NATMap{
		cache:   cache.New(natTimeout),
		Timeout: natTimeout,
	}
}

func NATMapInstance() *NATMap {
	once.Do(func() {
		natMap = newNATMap()
	})
	return natMap
}
