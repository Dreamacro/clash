package adapters

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	C "github.com/Dreamacro/clash/constant"
)

// HTTPAdapter is a proxy adapter
type HTTPAdapter struct {
	conn net.Conn
}

// Close is used to close connection
func (ha *HTTPAdapter) Close() {
	ha.conn.Close()
}

func (ha *HTTPAdapter) Conn() net.Conn {
	return ha.conn
}

type Http struct {
	addr           string
	name           string
	user           string
	pass           string
	tls            bool
	skipCertVerify bool
	tlsConfig      *tls.Config
}

type HttpOption struct {
	Name           string `proxy:"name"`
	Server         string `proxy:"server"`
	Port           int    `proxy:"port"`
	UserName       string `proxy:"username,omitempty"`
	Password       string `proxy:"password,omitempty"`
	TLS            bool   `proxy:"tls,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
}

func (h *Http) Name() string {
	return h.name
}

func (h *Http) Type() C.AdapterType {
	return C.Http
}

func (h *Http) Generator(metadata *C.Metadata) (adapter C.ProxyAdapter, err error) {
	c, err := net.DialTimeout("tcp", h.addr, tcpTimeout)
	if err == nil && h.tls {
		cc := tls.Client(c, h.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	if err != nil {
		return nil, fmt.Errorf("%s connect error", h.addr)
	}
	tcpKeepAlive(c)
	if err := h.shakeHand(metadata, c); err != nil {
		return nil, err
	}

	return &HTTPAdapter{conn: c}, nil
}

func (h *Http) shakeHand(metadata *C.Metadata, rw io.ReadWriter) error {
	var buf bytes.Buffer
	var err error

	buf.WriteString("CONNECT ")
	buf.WriteString(net.JoinHostPort(metadata.Host, metadata.Port))
	buf.WriteString(" HTTP/1.1\r\n")
	buf.WriteString("Proxy-Connection: Keep-Alive\r\n")

	if h.user != "" && h.pass != "" {
		auth := h.user + ":" + h.pass
		buf.WriteString("Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(auth)) + "\r\n")
	}
	// header ended
	buf.WriteString("\r\n")

	_, err = rw.Write(buf.Bytes())
	if err != nil {
		return err
	}

	var req http.Request
	resp, err := http.ReadResponse(bufio.NewReader(rw), &req)
	if err != nil {
		return err
	}

	if resp.StatusCode == 200 {
		return nil
	}

	if resp.StatusCode == 407 {
		return errors.New("HTTP need auth")
	}

	if resp.StatusCode == 405 {
		return errors.New("CONNECT method not allowed by proxy")
	}
	return fmt.Errorf("can not connect remote err code: %d", resp.StatusCode)
}

func (h *Http) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": h.Type().String(),
	})
}

func NewHttp(option HttpOption) *Http {
	var tlsConfig *tls.Config
	if option.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: option.SkipCertVerify,
			ClientSessionCache: getClientSessionCache(),
			MinVersion:         tls.VersionTLS11,
			MaxVersion:         tls.VersionTLS12,
			ServerName:         option.Server,
		}
	}

	return &Http{
		addr:           net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
		name:           option.Name,
		user:           option.UserName,
		pass:           option.Password,
		tls:            option.TLS,
		skipCertVerify: option.SkipCertVerify,
		tlsConfig:      tlsConfig,
	}
}
