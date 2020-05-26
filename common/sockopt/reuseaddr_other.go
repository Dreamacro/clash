// +build !linux

package sockopt

import (
	"net"
)

func UDPReuseaddr(c *net.UDPConn) (err error) {
	return
}
