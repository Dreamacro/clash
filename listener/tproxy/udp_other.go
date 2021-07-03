// +build !linux

package tproxy

import (
	"errors"
	"net"
)

func getOrigDst(oob []byte, oobn int) (*net.UDPAddr, error) {
	return nil, errors.New("UDP redir not supported on current platform")
}

func dialUDP(network string, lAddr *net.UDPAddr, rAddr *net.UDPAddr) (*net.UDPConn, error) {
	return nil, errors.New("UDP redir not supported on current platform")
}
