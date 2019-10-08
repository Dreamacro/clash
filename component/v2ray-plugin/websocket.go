package obfs

import (
	"crypto/tls"
	"net"
	"net/http"

	"github.com/Dreamacro/clash/component/vmess"
)

// Option is options of websocket obfs
type Option struct {
	Host      string
	Path      string
	Headers   map[string]string
	TLSConfig *tls.Config
	Mux       bool
}

// NewV2rayObfs return a HTTPObfs
func NewV2rayObfs(conn net.Conn, option *Option) (net.Conn, error) {
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

	if option.Mux {
		conn = NewMux(conn, MuxOption{
			ID:   [2]byte{0, 0},
			Host: "127.0.0.1",
			Port: 0,
		})
	}
	return conn, nil
}
