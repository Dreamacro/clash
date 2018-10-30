package adapters

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Dreamacro/clash/component/vmess"
	C "github.com/Dreamacro/clash/constant"
)

// VmessAdapter is a vmess adapter
type VmessAdapter struct {
	conn net.Conn
}

// Close is used to close connection
func (v *VmessAdapter) Close() {
	v.conn.Close()
}

func (v *VmessAdapter) Conn() net.Conn {
	return v.conn
}

type Vmess struct {
	name   string
	server string
	client *vmess.Client
}

type VmessOption struct {
	Name           string `proxy:"name"`
	Server         string `proxy:"server"`
	Port           int    `proxy:"port"`
	UUID           string `proxy:"uuid"`
	AlterID        int    `proxy:"alterId"`
	Cipher         string `proxy:"cipher"`
	TLS            bool   `proxy:"tls,omitempty"`
	Network        string `proxy:"network,omitempty"`
	WSPath         string `proxy:"ws-path,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
}

func (ss *Vmess) Name() string {
	return ss.name
}

func (ss *Vmess) Type() C.AdapterType {
	return C.Vmess
}

func (ss *Vmess) Generator(metadata *C.Metadata) (adapter C.ProxyAdapter, err error) {
	c, err := net.DialTimeout("tcp", ss.server, tcpTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s connect error", ss.server)
	}
	tcpKeepAlive(c)
	c, err = ss.client.New(c, parseVmessAddr(metadata))
	return &VmessAdapter{conn: c}, err
}

func NewVmess(option VmessOption) (*Vmess, error) {
	security := strings.ToLower(option.Cipher)
	client, err := vmess.NewClient(vmess.Config{
		UUID:           option.UUID,
		AlterID:        uint16(option.AlterID),
		Security:       security,
		TLS:            option.TLS,
		Host:           fmt.Sprintf("%s:%d", option.Server, option.Port),
		NetWork:        option.Network,
		WebSocketPath:  option.WSPath,
		SkipCertVerify: option.SkipCertVerify,
	})
	if err != nil {
		return nil, err
	}

	return &Vmess{
		name:   option.Name,
		server: fmt.Sprintf("%s:%d", option.Server, option.Port),
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
		copy(addr[:], metadata.IP.To4())
	case C.AtypIPv6:
		addrType = byte(vmess.AtypIPv6)
		addr = make([]byte, net.IPv6len)
		copy(addr[:], metadata.IP.To16())
	case C.AtypDomainName:
		addrType = byte(vmess.AtypDomainName)
		addr = make([]byte, len(metadata.Host)+1)
		addr[0] = byte(len(metadata.Host))
		copy(addr[1:], []byte(metadata.Host))
	}

	port, _ := strconv.Atoi(metadata.Port)
	return &vmess.DstAddr{
		AddrType: addrType,
		Addr:     addr,
		Port:     uint(port),
	}
}
