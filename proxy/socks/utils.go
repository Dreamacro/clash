package socks

import (
	"bytes"
	"net"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
)

type fakeConn struct {
	net.PacketConn
	remoteAddr net.Addr
	targetAddr socks5.Addr
	buffer     *bytes.Buffer
	bufRef     []byte
}

func (c *fakeConn) Read(b []byte) (n int, err error) {
	return c.buffer.Read(b)
}

func (c *fakeConn) Write(b []byte) (n int, err error) {
	packet, err := socks5.EncodeUDPPacket(c.targetAddr, b)
	if err != nil {
		return
	}
	return c.PacketConn.WriteTo(packet, c.remoteAddr)
}

func (c *fakeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *fakeConn) Close() error {
	err := c.PacketConn.Close()
	pool.BufPool.Put(c.bufRef[:cap(c.bufRef)])
	return err
}
