package sockopt

import (
	"net"
	"syscall"
)

func UDPReuseaddr(c *net.UDPConn) (err error) {
	rc, err := c.SyscallConn()
	if err != nil {
		return
	}

	rc.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})

	return
}
