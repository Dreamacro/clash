package adapters

import (
	"net"
	"net/http"

	C "github.com/Dreamacro/clash/constant"
)

// NewHTTPS is HTTPAdapter generator
func NewHTTPS(request *http.Request, conn net.Conn) *SocketAdapter {
	metadata := parseHTTPAddr(request)
	metadata.Type = C.HTTPCONNECT
	if ip, port, err := parseAddr(conn.RemoteAddr().String()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = port
	}
	return &SocketAdapter{
		metadata: metadata,
		Conn:     conn,
	}
}
