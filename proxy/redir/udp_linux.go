// +build linux

package redir

import (
	"encoding/binary"
	"errors"
	"net"
	"syscall"
)

const (
	IPV6_TRANSPARENT     = 0x4b
	IPV6_RECVORIGDSTADDR = 0x4a
)

func setsockopt(c *net.UDPConn, addr string) error {
	isIPv6 := true
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.To4() != nil {
		isIPv6 = false
	}

	rc, err := c.SyscallConn()
	if err != nil {
		return err
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

func getOrigDst(oob []byte, oobn int) (*net.UDPAddr, error) {
	msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, err
	}

	for _, msg := range msgs {
		if msg.Header.Level == syscall.SOL_IP && msg.Header.Type == syscall.IP_RECVORIGDSTADDR {
			ip := net.IP(msg.Data[4:8])
			port := binary.BigEndian.Uint16(msg.Data[2:4])
			return &net.UDPAddr{IP: ip, Port: int(port)}, nil
		} else if msg.Header.Level == syscall.SOL_IPV6 && msg.Header.Type == IPV6_RECVORIGDSTADDR {
			ip := net.IP(msg.Data[8:24])
			port := binary.BigEndian.Uint16(msg.Data[2:4])
			return &net.UDPAddr{IP: ip, Port: int(port)}, nil
		}
	}

	return nil, errors.New("cannot find origDst")
}
