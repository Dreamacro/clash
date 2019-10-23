package adapters

import (
	"context"
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type Direct struct {
	*Base
}

func (d *Direct) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	address := net.JoinHostPort(metadata.Host, metadata.DstPort)
	if metadata.DstIP != nil {
		address = net.JoinHostPort(metadata.DstIP.String(), metadata.DstPort)
	}

	c, err := dialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	tcpKeepAlive(c)
	return newConn(c, d), nil
}

func (d *Direct) DialUDP(metadata *C.Metadata) (C.PacketConn, net.Addr, error) {
	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, nil, err
	}

	addr, err := resolveUDPAddr("udp", metadata.RemoteAddress())
	if err != nil {
		return nil, nil, err
	}
	return newPacketConn(pc, d), addr, nil
}

func (d *Direct) HealthCheck(ctx context.Context, url string) (uint16, error) {
	return urlTest(ctx, d, url)
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
