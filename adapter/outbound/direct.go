package outbound

import (
	"context"
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type Direct struct {
	*Base
}

// DialContext implements C.ProxyAdapter
func (d *Direct) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return DialContextDecorated(ctx, network, address, func(conn net.Conn) (net.Conn, error) {
		return WithRouteHop(conn, d), nil
	})
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
