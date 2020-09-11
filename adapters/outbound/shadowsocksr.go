package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/ssr/obfs"
	"github.com/Dreamacro/clash/component/ssr/protocol"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/go-shadowsocks2/core"
	"github.com/Dreamacro/go-shadowsocks2/shadowstream"
)

type ShadowSocksR struct {
	*Base
	cipher   *core.StreamCipher
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
	SocketMark    string `proxy:"socket-mark",omitempty"`
	Interface     string `proxy:"interface-name,omitempty"`
}

func (ssr *ShadowSocksR) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	c = obfs.NewConn(c, ssr.obfs)
	c = ssr.cipher.StreamConn(c)
	conn, ok := c.(*shadowstream.Conn)
	if !ok {
		return nil, fmt.Errorf("invalid connection type")
	}
	iv, err := conn.ObtainWriteIV()
	if err != nil {
		return nil, err
	}
	c = protocol.NewConn(c, ssr.protocol, iv)
	_, err = c.Write(serializesSocksAddr(metadata))
	return c, err
}

func (ssr *ShadowSocksR) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", ssr.addr, dialer.DialOptions{SocketMark: ssr.SocketMark(), Interface: ssr.Interface()})
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", ssr.addr, err)
	}
	tcpKeepAlive(c)

	c, err = ssr.StreamConn(c, metadata)
	return NewConn(c, ssr), err
}

func (ssr *ShadowSocksR) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pc, err := dialer.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}

	addr, err := resolveUDPAddr("udp", ssr.addr)
	if err != nil {
		return nil, err
	}

	pc = ssr.cipher.PacketConn(pc)
	pc = protocol.NewPacketConn(pc, ssr.protocol)
	return newPacketConn(&ssPacketConn{PacketConn: pc, rAddr: addr}, ssr), nil
}

func (ssr *ShadowSocksR) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": ssr.Type().String(),
	})
}

func NewShadowSocksR(option ShadowSocksROption) (*ShadowSocksR, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	coreCiph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize cipher error: %w", addr, err)
	}
	ciph, ok := coreCiph.(*core.StreamCipher)
	if !ok {
		return nil, fmt.Errorf("%s is not a supported stream cipher in ssr", cipher)
	}

	obfs, err := obfs.PickObfs(option.Obfs, &obfs.Base{
		IVSize:  ciph.IVSize(),
		Key:     ciph.Key,
		HeadLen: 30,
		Host:    option.Server,
		Port:    option.Port,
		Param:   option.ObfsParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize obfs error: %w", addr, err)
	}

	protocol, err := protocol.PickProtocol(option.Protocol, &protocol.Base{
		IV:     nil,
		Key:    ciph.Key,
		TCPMss: 1460,
		Param:  option.ProtocolParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize protocol error: %w", addr, err)
	}
	protocol.SetOverhead(obfs.GetObfsOverhead() + protocol.GetProtocolOverhead())

	return &ShadowSocksR{
		Base: &Base{
			name:       option.Name,
			addr:       addr,
			tp:         C.ShadowsocksR,
			udp:        option.UDP,
			socketmark: option.SocketMark,
			ifname:     option.Interface,
		},
		cipher:   ciph,
		obfs:     obfs,
		protocol: protocol,
	}, nil
}
