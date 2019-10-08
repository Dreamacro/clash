package adapters

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	C "github.com/Dreamacro/clash/constant"
)

type Http struct {
	*Base
	addr           string
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

func (h *Http) Dial(metadata *C.Metadata) (C.Conn, error) {
	c, err := dialTimeout("tcp", h.addr, tcpTimeout)
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

	return newConn(c, h), nil
}

func (h *Http) shakeHand(metadata *C.Metadata, rw io.ReadWriter) error {
	var buf bytes.Buffer
	var err error

	addr := net.JoinHostPort(metadata.String(), metadata.DstPort)
	buf.WriteString("CONNECT " + addr + " HTTP/1.1\r\n")
	buf.WriteString("Host: " + metadata.String() + "\r\n")
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

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusProxyAuthRequired {
		return errors.New("HTTP need auth")
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		return errors.New("CONNECT method not allowed by proxy")
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return errors.New(resp.Status)
	}
	return fmt.Errorf("can not connect remote err code: %d", resp.StatusCode)
}

func NewHttp(option HttpOption) *Http {
	var tlsConfig *tls.Config
	if option.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: option.SkipCertVerify,
			ClientSessionCache: getClientSessionCache(),
			ServerName:         option.Server,
		}
	}

	return &Http{
		Base: &Base{
			name: option.Name,
			tp:   C.Http,
		},
		addr:           net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
		user:           option.UserName,
		pass:           option.Password,
		tls:            option.TLS,
		skipCertVerify: option.SkipCertVerify,
		tlsConfig:      tlsConfig,
	}
}
