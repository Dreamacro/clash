// +build linux

package tproxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
)

const (
	IPV6_TRANSPARENT     = 0x4b
	IPV6_RECVORIGDSTADDR = 0x4a
)

// dialUDP acts like net.DialUDP for transparent proxy.
// It binds to a non-local address(`lAddr`).
func dialUDP(network string, lAddr *net.UDPAddr, rAddr *net.UDPAddr) (*net.UDPConn, error) {
	rSockAddr, err := udpAddrToSockAddr(rAddr)
	if err != nil {
		return nil, err
	}

	lSockAddr, err := udpAddrToSockAddr(lAddr)
	if err != nil {
		return nil, err
	}

	fd, err := syscall.Socket(udpAddrFamily(network, lAddr, rAddr), syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = syscall.Bind(fd, lSockAddr); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = syscall.Connect(fd, rSockAddr); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	fdFile := os.NewFile(uintptr(fd), fmt.Sprintf("net-udp-dial-%s", rAddr.String()))
	defer fdFile.Close()

	c, err := net.FileConn(fdFile)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return c.(*net.UDPConn), nil
}

func udpAddrToSockAddr(addr *net.UDPAddr) (syscall.Sockaddr, error) {
	switch {
	case addr.IP.To4() != nil:
		ip := [4]byte{}
		copy(ip[:], addr.IP.To4())

		return &syscall.SockaddrInet4{Addr: ip, Port: addr.Port}, nil

	default:
		ip := [16]byte{}
		copy(ip[:], addr.IP.To16())

		zoneID, err := strconv.ParseUint(addr.Zone, 10, 32)
		if err != nil {
			zoneID = 0
		}

		return &syscall.SockaddrInet6{Addr: ip, Port: addr.Port, ZoneId: uint32(zoneID)}, nil
	}
}

func udpAddrFamily(net string, lAddr, rAddr *net.UDPAddr) int {
	switch net[len(net)-1] {
	case '4':
		return syscall.AF_INET
	case '6':
		return syscall.AF_INET6
	}

	if (lAddr == nil || lAddr.IP.To4() != nil) && (rAddr == nil || lAddr.IP.To4() != nil) {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
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
