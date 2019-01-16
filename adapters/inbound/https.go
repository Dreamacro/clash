package adapters

import (
	"net"
	"net/http"
)

// NewHTTPS is HTTPAdapter generator
func NewHTTPS(request *http.Request, conn net.Conn) *SocketAdapter {
	metadata := parseHTTPAddr(request)
	metadata.SourceIP = parseSourceIP(conn)
	return &SocketAdapter{
		metadata: metadata,
		conn:     conn,
	}
}
