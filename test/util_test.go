// +build !darwin

package main

import "net"

func defaultRouteIP() (net.IP, error) {
	return net.ParseIP("127.0.0.1"), nil
}
