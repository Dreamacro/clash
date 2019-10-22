package rules

import (
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type IPCIDR struct {
	ipnet      *net.IPNet
	adapter    string
	isSourceIP bool
	needIP		 bool
}

func (i *IPCIDR) RuleType() C.RuleType {
	if i.isSourceIP {
		return C.SrcIPCIDR
	}
	return C.IPCIDR
}

func (i *IPCIDR) IsMatch(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if !i.needIP {
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

func (i *IPCIDR) NeedIP() bool {
	return i.needIP
}

func NewIPCIDR(s string, adapter string, params []string, isSourceIP bool) *IPCIDR {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	return &IPCIDR{
		ipnet:      ipnet,
		adapter:    adapter,
		isSourceIP: isSourceIP,
		needIP: !HasParam(params, "no-resolve"),
	}
}
