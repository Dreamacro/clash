package nat

import (
	"net"
	"runtime"
	"sync"
	"time"
)

type Table struct {
	*table
}

type table struct {
	mapping sync.Map
	janitor *janitor
	timeout time.Duration
}

type element struct {
	Expired    time.Time
	RemoteAddr net.Addr
	RemoteConn net.PacketConn
}

func (t *table) Set(key net.Addr, rConn net.PacketConn, rAddr net.Addr) {
	// set conn read timeout
	rConn.SetReadDeadline(time.Now().Add(t.timeout))
	t.mapping.Store(key, &element{
		RemoteAddr: rAddr,
		RemoteConn: rConn,
		Expired:    time.Now().Add(t.timeout),
	})
}

func (t *table) Get(key net.Addr) (rConn net.PacketConn, rAddr net.Addr) {
	item, exist := t.mapping.Load(key)
	if !exist {
		return
	}
	elm := item.(*element)
	// expired
	if time.Since(elm.Expired) > 0 {
		t.mapping.Delete(key)
		elm.RemoteConn.Close()
		return
	}
	// reset expired time
	elm.Expired = time.Now().Add(t.timeout)
	return elm.RemoteConn, elm.RemoteAddr
}

func (t *table) cleanup() {
	t.mapping.Range(func(k, v interface{}) bool {
		key := k.(net.Addr)
		elm := v.(*element)
		if time.Since(elm.Expired) > 0 {
			t.mapping.Delete(key)
			elm.RemoteConn.Close()
		}
		return true
	})
}

type janitor struct {
	interval time.Duration
	stop     chan struct{}
}

func (j *janitor) process(t *table) {
	ticker := time.NewTicker(j.interval)
	for {
		select {
		case <-ticker.C:
			t.cleanup()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(t *Table) {
	t.janitor.stop <- struct{}{}
}

// New return *Cache
func New(interval time.Duration) *Table {
	j := &janitor{
		interval: interval,
		stop:     make(chan struct{}),
	}
	t := &table{janitor: j, timeout: interval}
	go j.process(t)
	T := &Table{t}
	runtime.SetFinalizer(T, stopJanitor)
	return T
}
