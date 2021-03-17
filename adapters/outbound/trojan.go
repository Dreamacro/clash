package outbound

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/gun"
	"github.com/Dreamacro/clash/component/trojan"
	C "github.com/Dreamacro/clash/constant"

	"golang.org/x/net/http2"
)

type Trojan struct {
	*Base
	instance *trojan.Trojan

	// for gun mux
	gunTLSConfig *tls.Config
	gunConfig    *gun.Config
	transport    *http2.Transport
}

type TrojanOption struct {
	Name           string      `proxy:"name"`
	Server         string      `proxy:"server"`
	Port           int         `proxy:"port"`
	Password       string      `proxy:"password"`
	ALPN           []string    `proxy:"alpn,omitempty"`
	SNI            string      `proxy:"sni,omitempty"`
	SkipCertVerify bool        `proxy:"skip-cert-verify,omitempty"`
	UDP            bool        `proxy:"udp,omitempty"`
	Network        string      `proxy:"network,omitempty"`
	GrpcOpts       GrpcOptions `proxy:"grpc-opts,omitempty"`
}

func (t *Trojan) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	var err error
	if t.transport != nil {
		c, err = gun.StreamGunWithConn(c, t.gunTLSConfig, t.gunConfig)
	} else {
		c, err = t.instance.StreamConn(c)
	}

	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}

	err = t.instance.WriteHeader(c, trojan.CommandTCP, serializesSocksAddr(metadata))
	return c, err
}

func (t *Trojan) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	// gun transport, TODO: Optimize mux dial code
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
	c, err = t.StreamConn(c, metadata)
	if err != nil {
		return nil, err
	}

	return NewConn(c, t), err
}

func (t *Trojan) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	// gun transport, TODO: Optimize mux dial code
	if t.transport != nil {
		c, err := gun.StreamGunWithTransport(t.transport, t.gunConfig)
		if err != nil {
			return nil, err
		}

		if err = t.instance.WriteHeader(c, trojan.CommandUDP, serializesSocksAddr(metadata)); err != nil {
			c.Close()
			return nil, err
		}

		pc := t.instance.PacketConn(c)
		return newPacketConn(pc, t), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	c, err := dialer.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}
	tcpKeepAlive(c)
	c, err = t.instance.StreamConn(c)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}

	err = t.instance.WriteHeader(c, trojan.CommandUDP, serializesSocksAddr(metadata))
	if err != nil {
		return nil, err
	}

	pc := t.instance.PacketConn(c)
	return newPacketConn(pc, t), err
}

func (t *Trojan) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": t.Type().String(),
	})
}

func NewTrojan(option TrojanOption) (*Trojan, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))

	tOption := &trojan.Option{
		Password:           option.Password,
		ALPN:               option.ALPN,
		ServerName:         option.Server,
		SkipCertVerify:     option.SkipCertVerify,
		ClientSessionCache: getClientSessionCache(),
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
			ClientSessionCache: getClientSessionCache(),
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
