package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"

	N "github.com/Dreamacro/clash/common/net"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/socks5"
	"github.com/Dreamacro/clash/transport/ssr/obfs"
	"github.com/Dreamacro/clash/transport/ssr/protocol"

	"github.com/Dreamacro/go-shadowsocks2/core"
	"github.com/Dreamacro/go-shadowsocks2/shadowaead"
	"github.com/Dreamacro/go-shadowsocks2/shadowstream"
)

type ShadowSocksR struct {
	*Base
	cipher   core.Cipher
	obfs     obfs.Obfs
	protocol protocol.Protocol
}

type ShadowSocksROption struct {
	Name          string `proxy:"name"`
	Server        string `proxy:"server"`
	Port          int    `proxy:"port"`
	Password      string `proxy:"password"`
	Cipher        string `proxy:"cipher"`
	Obfs          string `proxy:"obfs"`
	ObfsParam     string `proxy:"obfs-param,omitempty"`
	Protocol      string `proxy:"protocol"`
	ProtocolParam string `proxy:"protocol-param,omitempty"`
	UDP           bool   `proxy:"udp,omitempty"`
}

// DialContext implements C.ProxyAdapter
func (ssr *ShadowSocksR) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return DialContextDecorated(ctx, network, ssr.addr, func(conn net.Conn) (net.Conn, error) {
		if pc, ok := conn.(net.PacketConn); ok {
			addr, err := resolveUDPAddr(network, address)
			if err != nil {
				return nil, err
			}

			ssrAddr, err := resolveUDPAddr(network, ssr.addr)
			if err != nil {
				return nil, err
			}

			pc = ssr.cipher.PacketConn(pc)
			pc = ssr.protocol.PacketConn(pc)
			pc = &ssPacketConn{PacketConn: pc, rAddr: ssrAddr}

			return WithRouteHop(N.NewStreamPacketConn(pc, addr), ssr), nil
		}

		conn = ssr.obfs.StreamConn(conn)
		conn = ssr.cipher.StreamConn(conn)

		var (
			iv  []byte
			err error
		)
		switch c := conn.(type) {
		case *shadowstream.Conn:
			iv, err = c.ObtainWriteIV()
			if err != nil {
				return nil, err
			}
		case *shadowaead.Conn:
			return nil, fmt.Errorf("invalid connection type")
		}

		conn = ssr.protocol.StreamConn(conn, iv)

		_, err = conn.Write(socks5.ParseAddr(address))

		return WithRouteHop(conn, ssr), err
	})
}

func NewShadowSocksR(option ShadowSocksROption) (*ShadowSocksR, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	coreCiph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize error: %w", addr, err)
	}
	var (
		ivSize int
		key    []byte
	)
	if option.Cipher == "dummy" {
		ivSize = 0
		key = core.Kdf(option.Password, 16)
	} else {
		ciph, ok := coreCiph.(*core.StreamCipher)
		if !ok {
			return nil, fmt.Errorf("%s is not dummy or a supported stream cipher in ssr", cipher)
		}
		ivSize = ciph.IVSize()
		key = ciph.Key
	}

	obfs, obfsOverhead, err := obfs.PickObfs(option.Obfs, &obfs.Base{
		Host:   option.Server,
		Port:   option.Port,
		Key:    key,
		IVSize: ivSize,
		Param:  option.ObfsParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize obfs error: %w", addr, err)
	}

	protocol, err := protocol.PickProtocol(option.Protocol, &protocol.Base{
		Key:      key,
		Overhead: obfsOverhead,
		Param:    option.ProtocolParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize protocol error: %w", addr, err)
	}

	return &ShadowSocksR{
		Base: &Base{
			name: option.Name,
			addr: addr,
			tp:   C.ShadowsocksR,
			udp:  option.UDP,
		},
		cipher:   coreCiph,
		obfs:     obfs,
		protocol: protocol,
	}, nil
}
