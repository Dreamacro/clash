package rules

import (
	"strconv"

	C "github.com/Dreamacro/clash/constant"
)

type Port struct {
	adapter  string
	port     string
	isSource bool
}

func (p *Port) RuleType() C.RuleType {
	if p.isSource {
		return C.SrcPort
	}
	return C.DstPort
}

func (p *Port) Match(metadata *C.Metadata) bool {
	if p.isSource {
		return metadata.SrcPort == p.port
	}
	return metadata.DstPort == p.port
}

func (p *Port) Adapter() string {
	return p.adapter
}

func (p *Port) Payload() string {
	return p.port
}

func (p *Port) NoResolveIP() bool {
	return false
}

func NewPort(port string, adapter string, isSource bool) (*Port, error) {
	_, err := strconv.Atoi(port)
	if err != nil {
		return nil, errPayload
	}
	return &Port{
		adapter:  adapter,
		port:     port,
		isSource: isSource,
	}, nil
}
