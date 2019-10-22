package rules

import (
	"net"
)

func HasParam(ps []string, param string) bool {
	for _, p := range ps {
		if p == param {
			return true
		}
	}
	return false
}

func HostToIP(host string) *net.IP {
	ip := net.ParseIP(host)
	if ip != nil {
		return &ip
	}
	return nil
}