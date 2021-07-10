package rules

import (
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

type Process struct {
	adapter string
	process string
}

func (ps *Process) RuleType() C.RuleType {
	return C.Process
}

func (ps *Process) Match(metadata *C.Metadata) bool {
	return strings.EqualFold(metadata.Proc, ps.process)
}

func (ps *Process) Adapter() string {
	return ps.adapter
}

func (ps *Process) Payload() string {
	return ps.process
}

func (ps *Process) ShouldResolveIP() bool {
	return false
}

func NewProcess(process string, adapter string) (*Process, error) {
	return &Process{
		adapter: adapter,
		process: process,
	}, nil
}
