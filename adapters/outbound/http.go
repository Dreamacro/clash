package outbound

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
)

type Http struct {
	*Base
	user      string
	pass      string
	tlsConfig *tls.Config
}

type HttpOption struct {
	Name           string `proxy:"name"`
	Server         string `proxy:"server"`
	Port           int    `proxy:"port"`
	UserName       string `proxy:"username,omitempty"`
	Password       string `proxy:"password,omitempty"`
	TLS            bool   `proxy:"tls,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
	SocketMark     string `proxy:"socket-mark,omitempty"`
	Interface      string `proxy:"interface-name,omitempty"`
}

func (h *Http) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	if h.tlsConfig != nil {
		cc := tls.Client(c, h.tlsConfig)
		err := cc.Handshake()
		c = cc
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", h.addr, err)
		}
	}

	if err := h.shakeHand(metadata, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (h *Http) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", h.addr, dialer.DialOptions{SocketMark: h.SocketMark(), Interface: h.Interface()})
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", h.addr, err)
	}
	tcpKeepAlive(c)

	c, err = h.StreamConn(c, metadata)
	if err != nil {
		return nil, err
	}

	return NewConn(c, h), nil
}

func (h *Http) shakeHand(metadata *C.Metadata, rw io.ReadWriter) error {
	addr := metadata.RemoteAddress()
	req := &http.Request{
		Method: http.MethodConnect,
		URL: &url.URL{
			Host: addr,
		},
		Host: addr,
		Header: http.Header{
			"Proxy-Connection": []string{"Keep-Alive"},
		},
	}

	if h.user != "" && h.pass != "" {
		auth := h.user + ":" + h.pass
		req.Header.Add("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}

	if err := req.Write(rw); err != nil {
		return err
	}

	resp, err := http.ReadResponse(bufio.NewReader(rw), req)
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
			name:       option.Name,
			addr:       net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
			tp:         C.Http,
			socketmark: option.SocketMark,
			ifname:     option.Interface,
		},
		user:      option.UserName,
		pass:      option.Password,
		tlsConfig: tlsConfig,
	}
}
