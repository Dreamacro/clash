package rules

import (
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type IPCIDR struct {
	ipnet      *net.IPNet
	adapter    string
	isSourceIP bool
	isNeedIP   bool
}

var defaultIPCIDR = IPCIDR{
	isSourceIP: false,
	isNeedIP:   true,
}

type IPCIDROption func(*IPCIDR)

func WithIPCIDRIsSourceIP(b bool) IPCIDROption {
	return func(i *IPCIDR) {
		i.isSourceIP = b
	}
}

func WithIPCIDRIsNeedIP(b bool) IPCIDROption {
	return func(i *IPCIDR) {
		i.isNeedIP = b
	}
}

func (i *IPCIDR) RuleType() C.RuleType {
	if i.isSourceIP {
		return C.SrcIPCIDR
	}
	return C.IPCIDR
}

func (i *IPCIDR) IsMatch(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if !i.isNeedIP {
		ip = HostToIP(metadata.Host)
	}
	if i.isSourceIP {
		ip = metadata.SrcIP
	}
	return ip != nil && i.ipnet.Contains(*ip)
}

func (i *IPCIDR) Adapter() string {
	return i.adapter
}

func (i *IPCIDR) Payload() string {
	return i.ipnet.String()
}

func (i *IPCIDR) IsNeedIP() bool {
	return i.isNeedIP
}

func NewIPCIDR(s string, adapter string, opts ...IPCIDROption) *IPCIDR {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	ipcidr := defaultIPCIDR
	for _, o := range opts {
		o(&ipcidr)
	}
	ipcidr.ipnet = ipnet
	ipcidr.adapter = adapter
	return &ipcidr
}
