package outbound

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/Dreamacro/clash/transport/vmess"
	"net"
	"net/http"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/gun"
	"github.com/Dreamacro/clash/transport/trojan"

	"golang.org/x/net/http2"
)

type Trojan struct {
	*Base
	instance *trojan.Trojan
	option   *TrojanOption

	// for gun mux
	gunTLSConfig *tls.Config
	gunConfig    *gun.Config
	transport    *http2.Transport
}

type TrojanOption struct {
	Name           string            `proxy:"name"`
	Server         string            `proxy:"server"`
	Port           int               `proxy:"port"`
	Password       string            `proxy:"password"`
	ALPN           []string          `proxy:"alpn,omitempty"`
	SNI            string            `proxy:"sni,omitempty"`
	SkipCertVerify bool              `proxy:"skip-cert-verify,omitempty"`
	UDP            bool              `proxy:"udp,omitempty"`
	Network        string            `proxy:"network,omitempty"`
	GrpcOpts       GrpcOptions       `proxy:"grpc-opts,omitempty"`
	WSPath         string            `proxy:"ws-path,omitempty"`
	WSHeaders      map[string]string `proxy:"ws-headers,omitempty"`
}

// StreamConn implements C.ProxyAdapter
func (t *Trojan) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	var err error
	switch t.option.Network {
	case "ws":
		host, port, _ := net.SplitHostPort(t.addr)
		wsOpts := &vmess.WebsocketConfig{
			Host: host,
			Port: port,
			Path: t.option.WSPath,
		}

		if len(t.option.WSHeaders) != 0 {
			header := http.Header{}
			for key, value := range t.option.WSHeaders {
				header.Add(key, value)
			}
			wsOpts.Headers = header
		}

		wsOpts.TLS = true
		wsOpts.SkipCertVerify = t.option.SkipCertVerify
		wsOpts.ServerName = t.option.SNI

		c, err = vmess.StreamWebsocketConn(c, wsOpts)
	case "grpc":
		c, err = gun.StreamGunWithConn(c, t.gunTLSConfig, t.gunConfig)
	default:
		c, err = t.instance.StreamConn(c)
	}

	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}

	err = t.instance.WriteHeader(c, trojan.CommandTCP, serializesSocksAddr(metadata))
	return c, err
}

// DialContext implements C.ProxyAdapter
func (t *Trojan) DialContext(ctx context.Context, metadata *C.Metadata) (_ C.Conn, err error) {
	// gun transport
	if t.transport != nil {
		c, err := gun.StreamGunWithTransport(t.transport, t.gunConfig)
		if err != nil {
			return nil, err
		}

		if err = t.instance.WriteHeader(c, trojan.CommandTCP, serializesSocksAddr(metadata)); err != nil {
			c.Close()
			return nil, err
		}

		return NewConn(c, t), nil
	}

	c, err := dialer.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}
	tcpKeepAlive(c)

	defer safeConnClose(c, err)

	c, err = t.StreamConn(c, metadata)
	if err != nil {
		return nil, err
	}

	return NewConn(c, t), err
}

// DialUDP implements C.ProxyAdapter
func (t *Trojan) DialUDP(metadata *C.Metadata) (_ C.PacketConn, err error) {
	var c net.Conn

	// grpc transport
	if t.transport != nil {
		c, err = gun.StreamGunWithTransport(t.transport, t.gunConfig)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
		}
		defer safeConnClose(c, err)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), C.DefaultTCPTimeout)
		defer cancel()
		c, err = dialer.DialContext(ctx, "tcp", t.addr)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
		}
		defer safeConnClose(c, err)
		tcpKeepAlive(c)
		c, err = t.instance.StreamConn(c)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
		}
	}

	err = t.instance.WriteHeader(c, trojan.CommandUDP, serializesSocksAddr(metadata))
	if err != nil {
		return nil, err
	}

	pc := t.instance.PacketConn(c)
	return newPacketConn(pc, t), err
}

func NewTrojan(option TrojanOption) (*Trojan, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))

	tOption := &trojan.Option{
		Password:       option.Password,
		ALPN:           option.ALPN,
		ServerName:     option.Server,
		SkipCertVerify: option.SkipCertVerify,
	}

	if option.SNI != "" {
		tOption.ServerName = option.SNI
	}

	t := &Trojan{
		Base: &Base{
			name: option.Name,
			addr: addr,
			tp:   C.Trojan,
			udp:  option.UDP,
		},
		instance: trojan.New(tOption),
		option:   &option,
	}

	if option.Network == "grpc" {
		dialFn := func(network, addr string) (net.Conn, error) {
			c, err := dialer.DialContext(context.Background(), "tcp", t.addr)
			if err != nil {
				return nil, fmt.Errorf("%s connect error: %s", t.addr, err.Error())
			}
			tcpKeepAlive(c)
			return c, nil
		}

		tlsConfig := &tls.Config{
			NextProtos:         option.ALPN,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: tOption.SkipCertVerify,
			ServerName:         tOption.ServerName,
		}

		t.transport = gun.NewHTTP2Client(dialFn, tlsConfig)
		t.gunTLSConfig = tlsConfig
		t.gunConfig = &gun.Config{
			ServiceName: option.GrpcOpts.GrpcServiceName,
			Host:        tOption.ServerName,
		}
	}

	return t, nil
}
