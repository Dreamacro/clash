package adapters

import (
	"net"
	"time"
)

func dialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}
