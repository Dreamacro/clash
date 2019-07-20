package adapters

import (
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
