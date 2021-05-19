package outbound

import (
	"context"
	"errors"

	C "github.com/Dreamacro/clash/constant"
)

type Pass struct {
	*Base
}

// DialContext implements C.ProxyAdapter
func (r *Pass) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	return nil, errors.New("match Pass rule")
}

// DialUDP implements C.ProxyAdapter
func (r *Pass) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	return nil, errors.New("match Pass rule")
}

func NewPass() *Pass {
	return &Pass{
		Base: &Base{
			name: "PASS",
			tp:   C.Pass,
			udp:  true,
		},
	}
}
