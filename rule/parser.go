package rules

import (
	"fmt"

	C "github.com/Dreamacro/clash/constant"
)

func ParseRule(tp, payload, target string, params []string) (C.Rule, error) {
	var (
		parseErr error
		parsed   C.Rule
	)

	switch tp {
	case "DOMAIN":
		parsed = NewDomain(payload, target)
	case "DOMAIN-SUFFIX":
		parsed = NewDomainSuffix(payload, target)
	case "DOMAIN-KEYWORD":
		parsed = NewDomainKeyword(payload, target)
	case "GEOIP":
		noResolve := HasNoResolve(params)
		parsed = NewGEOIP(payload, target, noResolve)
	case "IP-CIDR", "IP-CIDR6":
		noResolve := HasNoResolve(params)
		parsed, parseErr = NewIPCIDR(payload, target, WithIPCIDRNoResolve(noResolve))
	case "SRC-IP-CIDR":
		parsed, parseErr = NewIPCIDR(payload, target, WithIPCIDRSourceIP(true), WithIPCIDRNoResolve(true))
	case "SRC-PORT":
		parsed, parseErr = NewPort(payload, target, true)
	case "DST-PORT":
		parsed, parseErr = NewPort(payload, target, false)
	case "PROCESS-NAME":
		parsed, parseErr = NewProcess(payload, target)
	case "MATCH":
		parsed = NewMatch(target)
	default:
		parseErr = fmt.Errorf("unsupported rule type %s", tp)
	}

	return parsed, parseErr
}
