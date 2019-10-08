package dns

import (
	"errors"
	"net"
	"strings"
)

var (
	errIPNotFound = errors.New("cannot found ip")
	errIPVersion  = errors.New("ip version error")
)

// ResolveIPv4 with a host, return ipv4
func ResolveIPv4(host string) (net.IP, error) {
	if node := DefaultHosts.Search(host); node != nil {
		if ip := node.Data.(net.IP).To4(); ip != nil {
			return ip, nil
		}
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if !strings.Contains(host, ":") {
			return ip, nil
		}
		return nil, errIPVersion
	}

	if DefaultResolver != nil {
		return DefaultResolver.ResolveIPv4(host)
	}

	ipAddrs, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	for _, ip := range ipAddrs {
		if ip4 := ip.To4(); ip4 != nil {
			return ip4, nil
		}
	}

	return nil, errIPNotFound
}

// ResolveIPv6 with a host, return ipv6
func ResolveIPv6(host string) (net.IP, error) {
	if node := DefaultHosts.Search(host); node != nil {
		if ip := node.Data.(net.IP).To16(); ip != nil {
			return ip, nil
		}
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if strings.Contains(host, ":") {
			return ip, nil
		}
		return nil, errIPVersion
	}

	if DefaultResolver != nil {
		return DefaultResolver.ResolveIPv6(host)
	}

	ipAddrs, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	for _, ip := range ipAddrs {
		if ip.To4() == nil {
			return ip, nil
		}
	}

	return nil, errIPNotFound
}

// ResolveIP with a host, return ip
func ResolveIP(host string) (net.IP, error) {
	if node := DefaultHosts.Search(host); node != nil {
		return node.Data.(net.IP), nil
	}

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
