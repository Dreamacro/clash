package outbound

import (
	"context"
	"net"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/resolver"
	C "github.com/Dreamacro/clash/constant"
)

type Direct struct {
	*Base
}

func (d *Direct) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	address := net.JoinHostPort(metadata.String(), metadata.DstPort)

	c, err := dialer.DialContext(ctx, "tcp", address)
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

func (dp *directPacketConn) WriteWithMetadata(p []byte, metadata *C.Metadata) (n int, err error) {
	if !metadata.Resolved() {
		ip, err := resolver.ResolveIP(metadata.Host)
		if err != nil {
			return 0, err
		}
		metadata.DstIP = ip
	}
	return dp.WriteTo(p, metadata.UDPAddr())
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
