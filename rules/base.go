package rules

import (
	"errors"
)

var (
	errPayload            = errors.New("payload error")
	errParams             = errors.New("params error")
	ErrPlatformNotSupport = errors.New("not support on this platform")
	ErrInvalidNetwork     = errors.New("invalid network")

	noResolve = "no-resolve"
	fullMatch = "full-match"
)

func HasNoResolve(params []string) bool {
	for _, p := range params {
		if p == noResolve {
			return true
		}
	}
	return false
}

func HasFullMatch(params []string) bool {
	for _, p := range params {
		if p == fullMatch {
			return true
		}
	}
	return false
}
