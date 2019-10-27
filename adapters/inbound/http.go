package adapters

import (
	"net"
	"net/http"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

// HTTPAdapter is a adapter for HTTP connection
type HTTPAdapter struct {
	net.Conn
	metadata *C.Metadata
	R        *http.Request
}

// Metadata return destination metadata
func (h *HTTPAdapter) Metadata() *C.Metadata {
	return h.metadata
}

// NewHTTP is HTTPAdapter generator
func NewHTTP(request *http.Request, conn net.Conn) *HTTPAdapter {
	metadata := parseHTTPAddr(request)
	metadata.Type = C.HTTP
	if ip, port, err := parseAddr(conn.RemoteAddr().String()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = port
	}
	return &HTTPAdapter{
		metadata: metadata,
		R:        request,
		Conn:     conn,
	}
}

// RemoveHopByHopHeaders remove hop-by-hop header
func RemoveHopByHopHeaders(header http.Header) {
	// Strip hop-by-hop header based on RFC:
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html#sec13.5.1
	// https://www.mnot.net/blog/2011/07/11/what_proxies_must_do

	header.Del("Proxy-Connection")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("TE")
	header.Del("Trailers")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")

	connections := header.Get("Connection")
	header.Del("Connection")
	if len(connections) == 0 {
		return
	}
	for _, h := range strings.Split(connections, ",") {
		header.Del(strings.TrimSpace(h))
	}
}
