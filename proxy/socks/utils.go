package socks

import (
	"bytes"
	"net"

	"github.com/Dreamacro/clash/component/socks5"
)

type fakeConn struct {
	net.PacketConn
	target     string
	remoteAddr net.Addr
	buffer     *bytes.Buffer
}

func newfakeConn(conn net.PacketConn, target string, remoteAddr net.Addr, buf []byte) *fakeConn {
	buffer := bytes.NewBuffer(buf)
	return &fakeConn{
		PacketConn: conn,
		target:     target,
		buffer:     buffer,
		remoteAddr: remoteAddr,
	}
}

func (c *fakeConn) Read(b []byte) (n int, err error) {
	return c.buffer.Read(b)
}

func (c *fakeConn) Write(b []byte) (n int, err error) {
	packet, err := socks5.EncodeUDPPacket(c.target, b)
	if err != nil {
		return
	}
	return c.PacketConn.WriteTo(packet, c.remoteAddr)
}

func (c *fakeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
