package outbound

import (
	"context"
	"net"

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
)

type Direct struct {
	*Base
}

type DirectOption struct {
	Name       string `proxy:"name"`
	SocketMark string `proxy:"socket-mark,omitempty"`
	Interface  string `proxy:"interface-name,omitempty"`
}

func (d *Direct) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	address := net.JoinHostPort(metadata.String(), metadata.DstPort)

	c, err := dialer.DialContext(ctx, "tcp", address, dialer.DialOptions{SocketMark: d.SocketMark(), Interface: d.Interface()})
	if err != nil {
		return nil, err
	}
	tcpKeepAlive(c)
	return NewConn(c, d), nil
}

func (d *Direct) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pc, err := dialer.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}
	return newPacketConn(&directPacketConn{pc}, d), nil
}

type directPacketConn struct {
	net.PacketConn
}

func NewDirectWithOption(option DirectOption) *Direct {
	return &Direct{
		Base: &Base{
			name:       option.Name,
			tp:         C.Direct,
			udp:        true,
			socketmark: option.SocketMark,
			ifname:     option.Interface,
		},
	}
}

func NewDirect() *Direct {
	return &Direct{
		Base: &Base{
			name: "DIRECT",
			tp:   C.Direct,
			udp:  true,
		},
	}
}
