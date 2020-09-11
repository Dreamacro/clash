package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/trojan"
	C "github.com/Dreamacro/clash/constant"
)

type Trojan struct {
	*Base
	instance *trojan.Trojan
}

type TrojanOption struct {
	Name           string   `proxy:"name"`
	Server         string   `proxy:"server"`
	Port           int      `proxy:"port"`
	Password       string   `proxy:"password"`
	ALPN           []string `proxy:"alpn,omitempty"`
	SNI            string   `proxy:"sni,omitempty"`
	SkipCertVerify bool     `proxy:"skip-cert-verify,omitempty"`
	UDP            bool     `proxy:"udp,omitempty"`
	SocketMark     string   `proxy:"socket-mark,omitempty"`
	Interface      string   `proxy:"interface-name,omitempty"`
}

func (t *Trojan) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	c, err := t.instance.StreamConn(c)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}

	err = t.instance.WriteHeader(c, trojan.CommandTCP, serializesSocksAddr(metadata))
	return c, err
}

func (t *Trojan) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", t.addr, dialer.DialOptions{SocketMark: t.SocketMark(), Interface: t.Interface()})
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
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	c, err := dialer.DialContext(ctx, "tcp", t.addr, dialer.DialOptions{SocketMark: t.SocketMark(), Interface: t.Interface()})
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

	return &Trojan{
		Base: &Base{
			name:       option.Name,
			addr:       addr,
			tp:         C.Trojan,
			udp:        option.UDP,
			socketmark: option.SocketMark,
			ifname:     option.Interface,
		},
		instance: trojan.New(tOption),
	}, nil
}
