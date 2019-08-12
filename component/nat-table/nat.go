package nat

import (
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/Dreamacro/clash/common/queue"
)

// NAT table is a simple map to store connections.
type Table struct {
	*table
}

type table struct {
	mapping sync.Map
	queue   *queue.Queue
	janitor *janitor
	timeout time.Duration
}

type element struct {
	Expired    time.Time
	RemoteAddr net.Addr
	RemoteConn net.PacketConn
}

// Set store network mapping to NAT table.
func (t *table) Set(key net.Addr, remoteConn net.PacketConn, rAddr net.Addr) {
	// set conn read timeout
	remoteConn.SetReadDeadline(time.Now().Add(t.timeout))
	t.mapping.Store(key, &element{
		RemoteAddr: rAddr,
		RemoteConn: remoteConn,
		Expired:    time.Now().Add(t.timeout),
	})
}

// Get return target network connection and address.
func (t *table) Get(key net.Addr) (remoteConn net.PacketConn, rAddr net.Addr) {
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

// AddConn put associate connections to queue
func (t *table) AddConn(conn net.Conn) {
	t.queue.Put(conn)
}

func (t *table) cleanup() {
	items := make([]interface{}, 0)
	queueLength := int(t.queue.Len())
	for i := 0; i < queueLength; i++ {
		items = append(items, t.queue.Pop())
	}

	var mapLength int
	t.mapping.Range(func(k, v interface{}) bool {
		key := k.(net.Addr)
		elm := v.(*element)
		if time.Since(elm.Expired) > 0 {
			t.mapping.Delete(key)
			elm.RemoteConn.Close()
		} else {
			mapLength += 1
		}
		return true
	})

	// if none active packet connection exists,
	// then close all tcp connections from queue.
	for _, item := range items {
		if mapLength != 0 {
			t.queue.Put(item)
		} else {
			if conn, ok := item.(net.Conn); ok {
				conn.Close()
			}
		}
	}
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

// New is a constructor for a new NAT table.
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
