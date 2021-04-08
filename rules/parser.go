package rules

import (
	"errors"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

func ParseUnitRule(tp, payload, target string) (C.Rule, error) {
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
		parsed = NewGEOIP(payload, target, false)
	case "IP-CIDR", "IP-CIDR6":
		parsed, parseErr = NewIPCIDR(payload, target, WithIPCIDRNoResolve(false))
	case "SRC-IP-CIDR":
		parsed, parseErr = NewIPCIDR(payload, target, WithIPCIDRSourceIP(true), WithIPCIDRNoResolve(true))
	case "PROTO":
		parsed, parseErr = NewProto(payload, target)
	case "SRC-PORT":
		parsed, parseErr = NewPort(payload, target, true)
	case "DST-PORT":
		parsed, parseErr = NewPort(payload, target, false)
	case "PROCESS-NAME":
		parsed, parseErr = NewProcess(payload, target)
	default:
		return nil, errNotRule
	}

	return parsed, parseErr
}

func ParseRule(rule string) (C.Rule, error) {
	units := []C.Rule{}
	tokenList := trimArr(strings.Split(rule, ","))

	if tokenList[0] == "MATCH" {
		return NewMatch(tokenList[1]), nil
	}

	i := 0
	for i < len(tokenList)-1 {
		unit, err := ParseUnitRule(tokenList[i], tokenList[i+1], "DUMMY")
		if err == nil {
			units = append(units, unit)
			i += 2
		} else if !errors.Is(err, errNotRule) {
			return nil, err
		} else {
			break
		}
	}
	target := tokenList[i]
	var params []string
	if len(tokenList[i:]) > 1 {
		params = tokenList[i+1:]
	}
	baseRule := NewBaseRule(units, target, params)

	return baseRule, nil
}

func trimArr(arr []string) (r []string) {
	for _, e := range arr {
		r = append(r, strings.Trim(e, " "))
	}
	return
}
