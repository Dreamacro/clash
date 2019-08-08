package constant

import (
	"context"
	"net"
	"time"
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
	LoadBalance
)

type ServerAdapter interface {
	net.Conn
	Metadata() *Metadata
}

type Connection interface {
	Chains() []string
	AppendToChains(adapter ProxyAdapter)
}

type Conn interface {
	net.Conn
	Connection
}

type PacketConn interface {
	net.PacketConn
	Connection
}

type ProxyAdapter interface {
	Name() string
	Type() AdapterType
	Dial(metadata *Metadata) (Conn, error)
	DialUDP(metadata *Metadata) (PacketConn, net.Addr, error)
	SupportUDP() bool
	Destroy()
	MarshalJSON() ([]byte, error)
}

type DelayHistory struct {
	Time  time.Time `json:"time"`
	Delay uint16    `json:"delay"`
}

type Proxy interface {
	ProxyAdapter
	Alive() bool
	DelayHistory() []DelayHistory
	LastDelay() uint16
	URLTest(ctx context.Context, url string) (uint16, error)
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
	case LoadBalance:
		return "LoadBalance"
	default:
		return "Unknow"
	}
}
