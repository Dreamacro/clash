package redir

import (
	"syscall"
	"unsafe"
)

const GETSOCKOPT = 15 // https://golang.org/src/syscall/syscall_linux_386.go#L183

func socketcall(call, a0, a1, a2, a3, a4, a5 uintptr) error {
	var a [6]uintptr
	a[0], a[1], a[2], a[3], a[4], a[5] = a0, a1, a2, a3, a4, a5
	if _, _, errno := syscall.Syscall6(syscall.SYS_SOCKETCALL, call, uintptr(unsafe.Pointer(&a)), 0, 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
