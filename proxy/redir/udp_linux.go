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
