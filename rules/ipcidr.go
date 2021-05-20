package rules

import (
	"net"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

type IPCIDROption func(*IPCIDR)

func WithIPCIDRSourceIP(b bool) IPCIDROption {
	return func(i *IPCIDR) {
		i.isSourceIP = b
	}
}

func WithIPCIDRNoResolve(noResolve bool) IPCIDROption {
	return func(i *IPCIDR) {
		i.noResolveIP = noResolve
	}
}

type IPCIDR struct {
	ipnet       *net.IPNet
	adapter     string
	isSourceIP  bool
	noResolveIP bool
}

func (i *IPCIDR) RuleType() C.RuleType {
	if i.isSourceIP {
		return C.SrcIPCIDR
	}
	return C.IPCIDR
}

func (i *IPCIDR) Match(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if i.isSourceIP {
		ip = metadata.SrcIP
	}
	return ip != nil && i.ipnet.Contains(ip)
}

func (i *IPCIDR) Adapter() string {
	return i.adapter
}

func (i *IPCIDR) Payload() string {
	return i.ipnet.String()
}

func (i *IPCIDR) ShouldResolveIP() bool {
	return !i.noResolveIP
}

const big = 0xFFFFFF

func dtoi(s string) (n int, i int, ok bool) {
	n = 0
	for i = 0; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n >= big {
			return big, i, false
		}
	}
	if i == 0 {
		return 0, 0, false
	}
	return n, i, true
}

func CIDRMask(ones, bits int, inver bool) net.IPMask {
	if bits != 8*net.IPv4len && bits != 8*net.IPv6len {
		return nil
	}
	if ones < 0 || ones > bits {
		return nil
	}
	l := bits / 8
	m := make(net.IPMask, l)
	n := uint(ones)

	u := byte(0xff)
	if inver {
		u = byte(0x00)
	}
	for i := 0; i < l; i++ {
		if n >= 8 {
			m[i] = u
			n -= 8
			continue
		}
		m[i] = ^byte(u >> n)
		n = 0
	}
	return m
}

func ParseCIDR(s string) (net.IP, *net.IPNet, error) {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return nil, nil, &net.ParseError{Type: "CIDR address", Text: s}
	}
	addr, mask := s[:i], s[i+1:]
	inver := false
	if strings.HasPrefix(mask, "-") {
		inver = true
		mask = mask[1:]
	}
	iplen := net.IPv4len
	ip := net.ParseIP(addr)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ':':
			iplen = net.IPv6len
		}
	}
	n, i, ok := dtoi(mask)
	if ip == nil || !ok || i != len(mask) || n < 0 || n > 8*iplen {
		return nil, nil, &net.ParseError{Type: "CIDR address", Text: s}
	}
	m := CIDRMask(n, 8*iplen, inver)
	return ip, &net.IPNet{IP: ip.Mask(m), Mask: m}, nil
}

func NewIPCIDR(s string, adapter string, opts ...IPCIDROption) (*IPCIDR, error) {
	_, ipnet, err := ParseCIDR(s)
	if err != nil {
		return nil, errPayload
	}

	ipcidr := &IPCIDR{
		ipnet:   ipnet,
		adapter: adapter,
	}

	for _, o := range opts {
		o(ipcidr)
	}

	return ipcidr, nil
}
