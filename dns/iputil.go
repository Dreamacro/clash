package dns

import (
	"errors"
	"net"
)

var (
	errIPNotFound = errors.New("cannot found ip")
)

// ResolveIP with a host, return ip
func ResolveIP(host string) (net.IP, error) {
	if DefaultResolver != nil {
		if DefaultResolver.ipv6 {
			return DefaultResolver.ResolveIP(host)
		}
		return DefaultResolver.ResolveIPv4(host)
	}

	ip := net.ParseIP(host)
	if ip != nil {
		return ip, nil
	}

	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, err
	}

	return ipAddr.IP, nil
}
