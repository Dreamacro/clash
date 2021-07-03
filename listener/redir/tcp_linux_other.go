// +build linux,!386

package redir

import "syscall"

const GETSOCKOPT = syscall.SYS_GETSOCKOPT

func socketcall(call, a0, a1, a2, a3, a4, a5 uintptr) error {
	if _, _, errno := syscall.Syscall6(call, a0, a1, a2, a3, a4, a5); errno != 0 {
		return errno
	}
	return nil
}
