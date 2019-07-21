package constant

import (
	"net"
)

// Socks addr type
const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4

	TCP NetWork = iota
	UDP

	HTTP Type = iota
	SOCKS
	REDIR
)

type NetWork int

func (n *NetWork) String() string {
	if *n == TCP {
		return "tcp"
	}
	return "udp"
}

type Type int

// Metadata is used to store connection address
type Metadata struct {
	NetWork  NetWork
	Type     Type
	SrcIP    *net.IP
	DstIP    *net.IP
	SrcPort  string
	DstPort  string
	AddrType int
	Host     string
}

func (m *Metadata) String() string {
	if m.Host == "" {
		return m.DstIP.String()
	}
	return m.Host
}

func (m *Metadata) Valid() bool {
	return m.Host != "" || m.DstIP != nil
}
