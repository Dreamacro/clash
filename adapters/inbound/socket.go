package adapters

import (
	"bytes"
	"net"

	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
)

// SocketAdapter is a adapter for socks and redir connection
type SocketAdapter struct {
	net.Conn
	metadata *C.Metadata
}

// Metadata return destination metadata
func (s *SocketAdapter) Metadata() *C.Metadata {
	return s.metadata
}

// NewSocket is SocketAdapter generator
func NewSocket(target socks5.Addr, conn net.Conn, source C.Type, netType C.NetWork) *SocketAdapter {
	metadata := parseSocksAddr(target)
	metadata.NetWork = netType
	metadata.Type = source
	if ip, port, err := parseAddr(conn.RemoteAddr().String()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = port
	}

	return &SocketAdapter{
		Conn:     conn,
		metadata: metadata,
	}
}

type FakeConn struct {
	net.PacketConn
	remoteAddr net.Addr
	buffer     *bytes.Buffer
}

func NewFakeConn(conn net.PacketConn, buf []byte, remoteAddr net.Addr) *FakeConn {
	var buffer *bytes.Buffer
	if buf != nil {
		buffer = bytes.NewBuffer(buf)
	}
	return &FakeConn{
		PacketConn: conn,
		buffer:     buffer,
		remoteAddr: remoteAddr,
	}
}

func (c *FakeConn) Read(b []byte) (n int, err error) {
	if c.buffer == nil {
		n, _, err = c.ReadFrom(b)
		return
	}
	return c.buffer.Read(b)
}

func (c *FakeConn) Write(b []byte) (n int, err error) {
	return c.PacketConn.WriteTo(b, c.remoteAddr)
}

func (c *FakeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
