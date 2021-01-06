package process

import (
	"errors"
	"net"
)

var (
	ErrInvalidNetwork     = errors.New("invalid network")
	ErrPlatformNotSupport = errors.New("not support on this platform")
	ErrNotFound           = errors.New("process not found")
)

const (
	TCP = "tcp"
	UDP = "udp"
)

func FindProcessName(network string, srcIP net.IP, srcPort int) (string, error) {
	return findProcessName(network, srcIP, srcPort)
}
