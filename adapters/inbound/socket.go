package adapters

import (
	"net"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/go-shadowsocks2/socks"
)

// SocketAdapter is a adapter for socks and redir connection
type SocketAdapter struct {
	conn     net.Conn
	metadata *C.Metadata
}

// Close socks and redir connection
func (s *SocketAdapter) Close() {
	s.conn.Close()
}

// Metadata return destination metadata
func (s *SocketAdapter) Metadata() *C.Metadata {
	return s.metadata
}

// Conn return raw net.Conn
func (s *SocketAdapter) Conn() net.Conn {
	return s.conn
}

// NewSocket is SocketAdapter generator
func NewSocket(target socks.Addr, conn net.Conn, source C.SourceType) *SocketAdapter {
	metadata := parseSocksAddr(target)
	metadata.Source = source
	metadata.SourceIP = parseSourceIP(conn)

	return &SocketAdapter{
		conn:     conn,
		metadata: metadata,
	}
}
