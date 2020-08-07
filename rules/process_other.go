// +build !darwin,!linux,!windows
// +build !freebsd !amd64

package rules

import (
	C "github.com/Dreamacro/clash/constant"
)

func NewProcess(process string, adapter string, fullMatch bool) (C.Rule, error) {
	return nil, ErrPlatformNotSupport
}
