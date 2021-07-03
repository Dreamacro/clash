// +build linux

package tproxy

import (
	"net"
	"syscall"
)

func setsockopt(rc syscall.RawConn, addr string) error {
	isIPv6 := true
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.To4() != nil {
		isIPv6 = false
	}

	rc.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)

		if err == nil {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
		}
		if err == nil && isIPv6 {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, IPV6_TRANSPARENT, 1)
		}

		if err == nil {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
		}
		if err == nil && isIPv6 {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, IPV6_RECVORIGDSTADDR, 1)
		}
	})

	return err
}
