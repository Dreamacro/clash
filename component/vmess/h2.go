package vmess

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/http2"
)

type h2Conn struct {
	net.Conn
	req     *http.Request
	pwriter *io.PipeWriter
	res     *http.Response
	cfg     *H2Config
}

type H2Config struct {
	Hosts []string
	Path  string
}

func (hc *h2Conn) establishConn() error {
	// TODO: use underlaying conn
	client := &http.Client{}
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	client.Transport = &http2.Transport{
		TLSClientConfig: tlsConfig,
	}

	preader, pwriter := io.Pipe()

	host := hc.cfg.Hosts[rand.Intn(len(hc.cfg.Hosts))]
	path := hc.cfg.Path
	// TODO: connect use VMess Host instead of H2 Host
	req := http.Request{
		Method: "PUT",
		Host:   host,
		URL: &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   path,
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Body:       preader,
		Header: map[string][]string{
			"Accept-Encoding": {"identity"},
		},
	}

	res, err := client.Do(&req)
	if err != nil {
		return err
	}

	hc.req = &req
	hc.res = res
	hc.pwriter = pwriter

	return nil
}

// Read implements net.Conn.Read()
func (hc *h2Conn) Read(b []byte) (int, error) {
	if hc.res != nil && hc.res.Close == false {
		n, err := hc.res.Body.Read(b)
		return n, err
	}

	if err := hc.establishConn(); err != nil {
		return 0, err
	}
	return hc.res.Body.Read(b)
}

// Write implements io.Writer.
func (hc *h2Conn) Write(b []byte) (int, error) {
	if hc.req != nil && hc.pwriter != nil && hc.res != nil && hc.res.Close == false {
		return hc.pwriter.Write(b)
	}

	if err := hc.establishConn(); err != nil {
		return 0, err
	}
	return hc.pwriter.Write(b)
}

func (hc *h2Conn) Close() error {
	if err := hc.pwriter.Close(); err != nil {
		return err
	}
	if err := hc.req.Body.Close(); err != nil {
		return err
	}
	return nil
}

func StreamH2Conn(conn net.Conn, cfg *H2Config) net.Conn {
	return &h2Conn{
		Conn: conn,
		cfg:  cfg,
	}
}
