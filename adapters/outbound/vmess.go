package adapters

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Dreamacro/clash/component/vmess"
	C "github.com/Dreamacro/clash/constant"
)

type Vmess struct {
	*Base
	server string
	client *vmess.Client
}

type VmessOption struct {
	Name           string            `proxy:"name"`
	Server         string            `proxy:"server"`
	Port           int               `proxy:"port"`
	UUID           string            `proxy:"uuid"`
	AlterID        int               `proxy:"alterId"`
	Cipher         string            `proxy:"cipher"`
	TLS            bool              `proxy:"tls,omitempty"`
	UDP            bool              `proxy:"udp,omitempty"`
	Network        string            `proxy:"network,omitempty"`
	WSPath         string            `proxy:"ws-path,omitempty"`
	WSHeaders      map[string]string `proxy:"ws-headers,omitempty"`
	SkipCertVerify bool              `proxy:"skip-cert-verify,omitempty"`
}

func (v *Vmess) Dial(metadata *C.Metadata) (net.Conn, error) {
	c, err := dialTimeout("tcp", v.server, tcpTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s connect error", v.server)
	}
	tcpKeepAlive(c)
	c, err = v.client.New(c, parseVmessAddr(metadata))
	return c, err
}

func (v *Vmess) DialUDP(metadata *C.Metadata) (net.PacketConn, net.Addr, error) {
	c, err := dialTimeout("tcp", v.server, tcpTimeout)
	if err != nil {
		return nil, nil, fmt.Errorf("%s connect error", v.server)
	}
	tcpKeepAlive(c)
	c, err = v.client.New(c, parseVmessAddr(metadata))
	if err != nil {
		return nil, nil, fmt.Errorf("new vmess client error: %v", err)
	}
	return &fakeUDPConn{Conn: c}, c.RemoteAddr(), nil
}

func NewVmess(option VmessOption) (*Vmess, error) {
	security := strings.ToLower(option.Cipher)
	client, err := vmess.NewClient(vmess.Config{
		UUID:             option.UUID,
		AlterID:          uint16(option.AlterID),
		Security:         security,
		TLS:              option.TLS,
		HostName:         option.Server,
		Port:             strconv.Itoa(option.Port),
		NetWork:          option.Network,
		WebSocketPath:    option.WSPath,
		WebSocketHeaders: option.WSHeaders,
		SkipCertVerify:   option.SkipCertVerify,
		SessionCache:     getClientSessionCache(),
	})
	if err != nil {
		return nil, err
	}

	return &Vmess{
		Base: &Base{
			name: option.Name,
			tp:   C.Vmess,
			udp:  true,
		},
		server: net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
		client: client,
	}, nil
}

func parseVmessAddr(metadata *C.Metadata) *vmess.DstAddr {
	var addrType byte
	var addr []byte
	switch metadata.AddrType {
	case C.AtypIPv4:
		addrType = byte(vmess.AtypIPv4)
		addr = make([]byte, net.IPv4len)
		copy(addr[:], metadata.DstIP.To4())
	case C.AtypIPv6:
		addrType = byte(vmess.AtypIPv6)
		addr = make([]byte, net.IPv6len)
		copy(addr[:], metadata.DstIP.To16())
	case C.AtypDomainName:
		addrType = byte(vmess.AtypDomainName)
		addr = make([]byte, len(metadata.Host)+1)
		addr[0] = byte(len(metadata.Host))
		copy(addr[1:], []byte(metadata.Host))
	}

	port, _ := strconv.Atoi(metadata.DstPort)
	return &vmess.DstAddr{
		UDP:      metadata.NetWork == C.UDP,
		AddrType: addrType,
		Addr:     addr,
		Port:     uint(port),
	}
}
