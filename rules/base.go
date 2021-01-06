package rules

import (
	"errors"
)

var (
	errPayload = errors.New("payload error")

	noResolve = "no-resolve"
)

func HasNoResolve(params []string) bool {
	for _, p := range params {
		if p == noResolve {
			return true
		}
	}
	return false
}
