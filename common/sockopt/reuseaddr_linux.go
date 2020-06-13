package sockopt

import (
	"net"
	"os/exec"
	"strings"
	"syscall"
)

func UDPReuseaddr(c *net.UDPConn) (err error) {
	rc, err := c.SyscallConn()
	if err != nil {
		return
	}

	rc.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		if err != nil && IsWSL() == true {
			err = nil
		}
	})

	return
}

func IsWSL() (isWSL bool) {
	cmd := exec.Command("uname", "-r")
	kernelInfo, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	if strings.Contains(string(kernelInfo), "Microsoft") {
		return true
	}

	return
}
