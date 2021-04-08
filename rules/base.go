package rules

import (
	"errors"
	"fmt"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

var (
	errPayload = errors.New("payload error")
	errNotRule = errors.New("not a rule")
	noResolve  = "no-resolve"
)

func HasNoResolve(params []string) bool {
	for _, p := range params {
		if p == noResolve {
			return true
		}
	}
	return false
}

type Base struct {
	units    []C.Rule
	inverses []bool
	params   []string
	adapter  string
}

func (b *Base) RuleType() C.RuleType {
	return C.Base
}

func (b *Base) Match(metadata *C.Metadata) bool {
	for i, unit := range b.units {
		if !unit.Match(metadata) && !b.inverses[i] {
			return false
		}
	}
	return true
}
func (b *Base) Payload() string {
	s := []string{}
	for i, unit := range b.units {
		if b.inverses[i] {
			s = append(s, fmt.Sprintf("%s(not %s)", unit.RuleType(), unit.Payload()))
		} else {
			s = append(s, fmt.Sprintf("%s(%s)", unit.RuleType(), unit.Payload()))
		}

	}
	p := strings.Join(s, " ")
	return p
}

func (b *Base) ShouldResolveIP() bool {
	if HasNoResolve(b.params) {
		return false
	}
	for _, unit := range b.units {
		if unit.ShouldResolveIP() {
			return true
		}
	}

	return false
}

func (b *Base) Adapter() string {
	return b.adapter
}
