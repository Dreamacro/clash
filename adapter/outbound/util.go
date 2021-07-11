package outbound

import (
	"errors"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/resolver"
)

func resolveUDPAddr(network, address string) (*net.UDPAddr, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	var ip net.IP
	switch network {
	case "udp":
		ip, err = resolver.ResolveIP(host)
	case "udp4":
		ip, err = resolver.ResolveIPv4(host)
	case "udp6":
		ip, err = resolver.ResolveIPv6(host)
	default:
		err = errors.New("invalid network")
	}
	if err != nil {
		return nil, err
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	return &net.UDPAddr{
		IP:   ip,
		Port: p,
		Zone: "",
	}, nil
}

func safeConnClose(c net.Conn, err error) {
	if err != nil {
		c.Close()
	}
}
