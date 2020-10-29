// +build !linux

package redir

import (
	"errors"
	"net"
)

func getOrigDst(oob []byte, oobn int) (*net.UDPAddr, error) {
	return nil, errors.New("UDP redir not supported on current platform")
}
