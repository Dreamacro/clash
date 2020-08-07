package rules

import (
	"unsafe"

	"github.com/Dreamacro/clash/common/cache"
	C "github.com/Dreamacro/clash/constant"
)

// store process name for when dealing with multiple PROCESS-NAME rules
var processCache = cache.NewLRUCache(cache.WithAge(2), cache.WithSize(64))

type Process struct {
	adapter   string
	process   string
	fullMatch bool
}

func (p *Process) RuleType() C.RuleType {
	return C.Process
}

func (p *Process) Adapter() string {
	return p.adapter
}

func (p *Process) Payload() string {
	return p.process
}

func (p *Process) ShouldResolveIP() bool {
	return false
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}
