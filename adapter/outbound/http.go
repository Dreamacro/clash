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
	SNI            string `proxy:"sni,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
}

// DialContext implements C.ProxyAdapter
func (h *Http) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return DialContextDecorated(ctx, network, h.addr, func(conn net.Conn) (net.Conn, error) {
		if _, ok := conn.(net.PacketConn); ok {
			return nil, fmt.Errorf("unsupported network: %s", network)
		}

		if h.tlsConfig != nil {
			c := tls.Client(conn, h.tlsConfig)
			err := c.Handshake()
			if err != nil {
				return nil, err
			}

			conn = c
		}

		err := h.shakeHand(address, conn)
		if err != nil {
			return nil, err
		}

		return WithRouteHop(conn, h), nil
	})
}

func (h *Http) shakeHand(address string, rw io.ReadWriter) error {
	req := &http.Request{
		Method: http.MethodConnect,
		URL: &url.URL{
			Host: address,
		},
		Host: address,
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
		sni := option.Server
		if option.SNI != "" {
			sni = option.SNI
		}
		tlsConfig = &tls.Config{
			InsecureSkipVerify: option.SkipCertVerify,
			ServerName:         sni,
		}
	}

	return &Http{
		Base: &Base{
			name: option.Name,
			addr: net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
			tp:   C.Http,
		},
		user:      option.UserName,
		pass:      option.Password,
		tlsConfig: tlsConfig,
	}
}
