package obfs

import (
	"crypto/tls"
	"net"
	"net/http"

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
	header := http.Header{}
	for k, v := range option.Headers {
		header.Add(k, v)
	}

	config := &vmess.WebsocketConfig{
		Host:      option.Host,
		Path:      option.Path,
		TLS:       option.TLSConfig != nil,
		Headers:   header,
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
