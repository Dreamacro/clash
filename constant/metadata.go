package constant

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/component/process"
	"github.com/Dreamacro/clash/log"
)

var processCache = cache.NewLRUCache(cache.WithAge(2), cache.WithSize(64))

// Socks addr type
const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4

	TCP NetWork = iota
	UDP

	HTTP Type = iota
	HTTPCONNECT
	SOCKS
	REDIR
	TPROXY
)

type NetWork int

func (n NetWork) String() string {
	if n == TCP {
		return "tcp"
	}
	return "udp"
}

func (n NetWork) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

type Type int

func (t Type) String() string {
	switch t {
	case HTTP:
		return "HTTP"
	case HTTPCONNECT:
		return "HTTP Connect"
	case SOCKS:
		return "Socks5"
	case REDIR:
		return "Redir"
	case TPROXY:
		return "TProxy"
	default:
		return "Unknown"
	}
}

func (t Type) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// Metadata is used to store connection address
type Metadata struct {
	NetWork  NetWork `json:"network"`
	Type     Type    `json:"type"`
	SrcIP    net.IP  `json:"sourceIP"`
	DstIP    net.IP  `json:"destinationIP"`
	SrcPort  string  `json:"sourcePort"`
	DstPort  string  `json:"destinationPort"`
	AddrType int     `json:"-"`
	Host     string  `json:"host"`
	Proc     string  `json:"processName"`
}

func (m *Metadata) RemoteAddress() string {
	return net.JoinHostPort(m.String(), m.DstPort)
}

func (m *Metadata) SourceAddress() string {
	return net.JoinHostPort(m.SrcIP.String(), m.SrcPort)
}

func (m *Metadata) Resolved() bool {
	return m.DstIP != nil
}

func (m *Metadata) UDPAddr() *net.UDPAddr {
	if m.NetWork != UDP || m.DstIP == nil {
		return nil
	}
	port, _ := strconv.Atoi(m.DstPort)
	return &net.UDPAddr{
		IP:   m.DstIP,
		Port: port,
	}
}

func (m *Metadata) String() string {
	if m.Host != "" {
		return m.Host
	} else if m.DstIP != nil {
		return m.DstIP.String()
	} else {
		return "<nil>"
	}
}

func (m *Metadata) Valid() bool {
	return m.Host != "" || m.DstIP != nil
}

func (m *Metadata) ReadProcessName() {
	key := fmt.Sprintf("%s:%s:%s", m.NetWork.String(), m.SrcIP.String(), m.SrcPort)
	cached, hit := processCache.Get(key)
	if !hit {
		srcPort, err := strconv.Atoi(m.SrcPort)
		if err != nil {
			processCache.Set(key, "<CannotRead>")
			m.Proc = "<CannotRead>"
			return
		}

		name, err := process.FindProcessName(m.NetWork.String(), m.SrcIP, srcPort)
		if err != nil {
			log.Debugln("[Rule] find process name %s error: %s", Process.String(), err.Error())
		}

		processCache.Set(key, name)

		cached = name
	}
	m.Proc = cached.(string)
}
