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

	HTTP SourceType = iota
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

type SourceType int

// Metadata is used to store connection address
type Metadata struct {
	NetWork  NetWork
	Source   SourceType
	AddrType int
	Host     string
	IP       *net.IP
	Port     string
}

func (addr *Metadata) String() string {
	if addr.Host == "" {
		return addr.IP.String()
	}
	return addr.Host
}
