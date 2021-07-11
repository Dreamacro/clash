package outbound

import (
	"encoding/json"

	C "github.com/Dreamacro/clash/constant"
)

type Base struct {
	name string
	addr string
	tp   C.AdapterType
	udp  bool
}

// Name implements C.ProxyAdapter
func (b *Base) Name() string {
	return b.name
}

// Type implements C.ProxyAdapter
func (b *Base) Type() C.AdapterType {
	return b.tp
}

// SupportUDP implements C.ProxyAdapter
func (b *Base) SupportUDP() bool {
	return b.udp
}

// MarshalJSON implements C.ProxyAdapter
func (b *Base) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": b.Type().String(),
	})
}

// Addr implements C.ProxyAdapter
func (b *Base) Addr() string {
	return b.addr
}

func NewBase(name string, addr string, tp C.AdapterType, udp bool) *Base {
	return &Base{name, addr, tp, udp}
}
