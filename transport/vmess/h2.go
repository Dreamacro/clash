package vmess

import (
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/http2"
)

type h2Conn struct {
	net.Conn
	*http2.ClientConn
	pwriter *io.PipeWriter
	res     *http.Response
	cfg     *H2Config
}

type H2Config struct {
	Hosts []string
	Path  string
}

func (hc *h2Conn) establishConn() error {
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

	// it will be close at :  `func (hc *h2Conn) Close() error`
	res, err := hc.ClientConn.RoundTrip(&req)
	if err != nil {
		return err
	}

	hc.pwriter = pwriter
	hc.res = res

	return nil
}

// Read implements net.Conn.Read()
func (hc *h2Conn) Read(b []byte) (int, error) {
	if hc.res != nil && !hc.res.Close {
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
	if hc.pwriter != nil {
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
	if err := hc.ClientConn.Shutdown(hc.res.Request.Context()); err != nil {
		return err
	}
	return hc.Conn.Close()
}

func StreamH2Conn(conn net.Conn, cfg *H2Config) (net.Conn, error) {
	transport := &http2.Transport{}

	cconn, err := transport.NewClientConn(conn)
	if err != nil {
		return nil, err
	}

	return &h2Conn{
		Conn:       conn,
		ClientConn: cconn,
		cfg:        cfg,
	}, nil
}
