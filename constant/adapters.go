package constant

import (
	"net"
)

// Adapter Type
const (
	Direct AdapterType = iota
	Fallback
	Reject
	Selector
	Shadowsocks
	Socks5
	Http
	URLTest
	Vmess
)

type ProxyAdapter interface {
	Conn() net.Conn
	Close()
}

type ServerAdapter interface {
	Metadata() *Metadata
	Close()
}

type Proxy interface {
	Name() string
	Type() AdapterType
	Generator(metadata *Metadata) (ProxyAdapter, error)
	MarshalJSON() ([]byte, error)
}

// AdapterType is enum of adapter type
type AdapterType int

func (at AdapterType) String() string {
	switch at {
	case Direct:
		return "Direct"
	case Fallback:
		return "Fallback"
	case Reject:
		return "Reject"
	case Selector:
		return "Selector"
	case Shadowsocks:
		return "Shadowsocks"
	case Socks5:
		return "Socks5"
	case Http:
		return "Http"
	case URLTest:
		return "URLTest"
	case Vmess:
		return "Vmess"
	default:
		return "Unknow"
	}
}
