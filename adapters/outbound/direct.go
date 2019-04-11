package adapters

import (
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type Direct struct {
	*Base
}

func (d *Direct) Dial(metadata *C.Metadata) (net.Conn, error) {
	address := net.JoinHostPort(metadata.Host, metadata.Port)
	if metadata.IP != nil {
		address = net.JoinHostPort(metadata.IP.String(), metadata.Port)
	}

	c, err := dialTimeout("tcp", address, tcpTimeout)
	if err != nil {
		return nil, err
	}
	tcpKeepAlive(c)
	return c, nil
}

func NewDirect() *Direct {
	return &Direct{
		Base: &Base{
			name: "DIRECT",
			tp:   C.Direct,
		},
	}
}
