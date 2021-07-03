package vmess

import (
	"crypto/tls"
	"net"
)

type TLSConfig struct {
	Host           string
	SkipCertVerify bool
	NextProtos     []string
}

func StreamTLSConn(conn net.Conn, cfg *TLSConfig) (net.Conn, error) {
	tlsConfig := &tls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: cfg.SkipCertVerify,
		NextProtos:         cfg.NextProtos,
	}

	tlsConn := tls.Client(conn, tlsConfig)
	err := tlsConn.Handshake()
	return tlsConn, err
}
