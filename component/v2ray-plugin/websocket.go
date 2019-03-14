package obfs

import (
	"crypto/tls"
	"net"

	"github.com/Dreamacro/clash/component/vmess"
)

// WebsocketOption is options of websocket obfs
type WebsocketOption struct {
	Host      string
	Path      string
	Headers   map[string]string
	TLSConfig *tls.Config
}

// NewWebsocketObfs return a HTTPObfs
func NewWebsocketObfs(conn net.Conn, option *WebsocketOption) (net.Conn, error) {
	config := &vmess.WebsocketConfig{
		Host:      option.Host,
		Path:      option.Path,
		TLS:       option.TLSConfig != nil,
		Headers:   option.Headers,
		TLSConfig: option.TLSConfig,
	}

	var err error
	conn, err = vmess.NewWebsocketConn(conn, config)
	if err != nil {
		return nil, err
	}
	conn = NewMux(conn, MuxOption{
		ID:   [2]byte{0, 0},
		Host: "127.0.0.1",
		Port: 0,
	})
	return conn, nil
}
