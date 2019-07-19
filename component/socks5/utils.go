package socks5

import (
	"bytes"
	"net"
	"sync"
	"time"
)

type UDPConn struct {
	net.PacketConn
	remoteAddr net.Addr
	buffer     *bytes.Buffer
}

func NewUDPConn(conn net.PacketConn, buf []byte, remoteAddr net.Addr) *UDPConn {
	var buffer *bytes.Buffer
	if buf != nil {
		buffer = bytes.NewBuffer(buf)
	}
	return &UDPConn{
		PacketConn: conn,
		buffer:     buffer,
		remoteAddr: remoteAddr,
	}
}

func (c *UDPConn) Read(b []byte) (n int, err error) {
	if c.buffer == nil {
		n, _, err = c.ReadFrom(b)
		return
	}
	return c.buffer.Read(b)
}

func (c *UDPConn) Write(b []byte) (n int, err error) {
	return c.PacketConn.WriteTo(b, c.remoteAddr)
}

func (c *UDPConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Packet NAT table
type NATMap struct {
	sync.RWMutex
	m       map[string]*UDPConn
	Timeout time.Duration
}

func NewNATMap(timeout time.Duration) *NATMap {
	m := &NATMap{}
	m.m = make(map[string]*UDPConn)
	m.Timeout = timeout
	return m
}

func (m *NATMap) Get(key string) *UDPConn {
	m.RLock()
	defer m.RUnlock()
	return m.m[key]
}

func (m *NATMap) Set(key string, pc *UDPConn) {
	m.Lock()
	defer m.Unlock()

	m.m[key] = pc
}

func (m *NATMap) Del(key string) *UDPConn {
	m.Lock()
	defer m.Unlock()

	pc, ok := m.m[key]
	if ok {
		delete(m.m, key)
		return pc
	}
	return nil
}
