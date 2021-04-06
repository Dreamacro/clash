package rules

import (
	C "github.com/Dreamacro/clash/constant"
)

type Proto struct {
	adapter string
	network C.NetWork
}

func (p *Proto) RuleType() C.RuleType {
	return C.Proto
}

func (p *Proto) Match(metadata *C.Metadata) bool {
	return metadata.NetWork == p.network
}

func (p *Proto) Adapter() string {
	return p.adapter
}

func (p *Proto) Payload() string {
	return p.network.String()
}

func (p *Proto) ShouldResolveIP() bool {
	return false
}

func NewProto(s, adapter string) (*Proto, error) {
	var network C.NetWork
	switch s {
	case "tcp":
		network = C.TCP
	case "udp":
		network = C.UDP
	default:
		return nil, errPayload
	}
	return &Proto{
		network: network,
		adapter: adapter,
	}, nil
}
