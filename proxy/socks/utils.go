package socks

import (
	"bytes"
	"net"
)

type fakeConn struct {
	net.PacketConn
	remoteAddr net.Addr
	buffer     *bytes.Buffer
}

func newfakeConn(conn net.PacketConn, buf []byte, remoteAddr net.Addr) *fakeConn {
	buffer := bytes.NewBuffer(buf)
	return &fakeConn{
		PacketConn: conn,
		buffer:     buffer,
		remoteAddr: remoteAddr,
	}
}

func (c *fakeConn) Read(b []byte) (n int, err error) {
	return c.buffer.Read(b)
}

func (c *fakeConn) Write(b []byte) (n int, err error) {
	return c.PacketConn.WriteTo(b, c.remoteAddr)
}

func (c *fakeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
