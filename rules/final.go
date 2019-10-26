package rules

import (
	C "github.com/Dreamacro/clash/constant"
)

type Match struct {
	adapter string
}

func (f *Match) RuleType() C.RuleType {
	return C.MATCH
}

func (f *Match) Match(metadata *C.Metadata) bool {
	return true
}

func (f *Match) Adapter() string {
	return f.adapter
}

func (f *Match) Payload() string {
	return ""
}

func (f *Match) NoResolveIP() bool {
	return false
}

func NewMatch(adapter string) *Match {
	return &Match{
		adapter: adapter,
	}
}
