package outbound

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	N "github.com/Dreamacro/clash/common/net"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/gun"
	"github.com/Dreamacro/clash/transport/socks5"
	"github.com/Dreamacro/clash/transport/trojan"
)

type Trojan struct {
	*Base
	instance *trojan.Trojan
	gun      *gun.Pool // for gun mux
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

// DialContext implements C.ProxyAdapter
func (t *Trojan) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var conn net.Conn

	// gun
	if t.gun != nil {
		trunk, err := t.gun.GetTrunk(ctx)
		if err != nil {
			return nil, err
		}

		conn, err = trunk.NewConn(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		c, err := DialContext(ctx, "tcp", t.addr)
		if err != nil {
			return nil, err
		}

		conn, err = t.instance.StreamConn(c)
		if err != nil {
			c.Close()

			return nil, err
		}
	}

	switch network {
	case "udp", "udp4", "udp6":
		addr, err := resolveUDPAddr(network, address)
		if err != nil {
			conn.Close()

			return nil, err
		}

		if err := t.instance.WriteHeader(conn, trojan.CommandUDP, socks5.ParseAddrToSocksAddr(addr)); err != nil {
			conn.Close()

			return nil, err
		}

		conn = N.NewStreamPacketConn(t.instance.PacketConn(conn), addr)
	default:
		if err := t.instance.WriteHeader(conn, trojan.CommandTCP, socks5.ParseAddr(address)); err != nil {
			conn.Close()

			return nil, err
		}
	}

	return WithRouteHop(conn, t), nil
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
	}

	if option.Network == "grpc" {
		tlsConfig := &tls.Config{
			NextProtos:         option.ALPN,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: tOption.SkipCertVerify,
			ServerName:         tOption.ServerName,
		}
		gunConfig := &gun.Config{
			ServiceName: option.GrpcOpts.GrpcServiceName,
			Host:        tOption.ServerName,
		}

		g := gun.New(tlsConfig, gunConfig)

		t.gun = gun.NewPool(func(ctx context.Context) (*gun.Trunk, error) {
			conn, err := DialContext(ctx, "tcp", t.addr)
			if err != nil {
				return nil, fmt.Errorf("%s connect error: %s", t.addr, err.Error())
			}

			trunk, err := g.NewTrunk(conn)
			if err != nil {
				conn.Close()

				return nil, err
			}

			return trunk, nil
		})
	}

	return t, nil
}
