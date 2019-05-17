package adapters

import (
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type Direct struct {
	*Base
}

func (d *Direct) Dial(metadata *C.Metadata) (C.Conn, error) {
	address := net.JoinHostPort(metadata.Host, metadata.DstPort)
	if metadata.DstIP != nil {
		address = net.JoinHostPort(metadata.DstIP.String(), metadata.DstPort)
	}

	c, err := net.DialTimeout("tcp", address, tcpTimeout)
	if err != nil {
		return nil, err
	}
	tcpKeepAlive(c)
	return NewConn(c, d), nil
}

func (d *Direct) DialUDP(metadata *C.Metadata) (net.PacketConn, net.Addr, error) {
	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, nil, err
	}

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(metadata.String(), metadata.DstPort))
	if err != nil {
		return nil, nil, err
	}
	return pc, addr, nil
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
