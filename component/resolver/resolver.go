package resolver

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	trie "github.com/Dreamacro/clash/component/domain-trie"
	"github.com/Dreamacro/clash/constant"
)

var (
	// DefaultResolver aim to resolve ip
	DefaultResolver Resolver

	// DefaultHosts aim to resolve hosts
	DefaultHosts = trie.New()

	hostsCache = cache.NewLRUCache(cache.WithSize(500))
)

var (
	ErrIPNotFound = errors.New("couldn't find ip")
	ErrIPVersion  = errors.New("ip version error")
)

type Resolver interface {
	ResolveIP(host string) (ip net.IP, err error)
	ResolveIPv4(host string) (ip net.IP, err error)
	ResolveIPv6(host string) (ip net.IP, err error)
}

// ResolveIPFromHosts randomly resolve IP from user given hosts bindings
func ResolveIPFromHosts(host string, ipGen int) net.IP {
	var ips []net.IP
	if cached, ok := hostsCache.Get(host + string(ipGen)); ok {
		ips = cached.([]net.IP)
		result := ips[int(time.Now().Unix()/30)%len(ips)]
		return result
	}

	node := DefaultHosts.Search(host)
	if node == nil {
		return nil
	}

	ips = node.Data.([]net.IP)
	if ipGen != 0 {
		searched := make([]net.IP, 0)
		for _, ip := range ips {
			if ipGen == constant.AtypIPv4 && ip.To4() != nil {
				searched = append(searched, ip)
			}

			if ipGen == constant.AtypIPv6 && ip.To4() == nil {
				searched = append(searched, ip)
			}
		}
		ips = searched
	}

	if len(ips) != 0 {
		hostsCache.Set(host+string(ipGen), ips)
	}

	result := ips[int(time.Now().Unix()/30)%len(ips)]
	return result
}

// ResolveIPv4 with a host, return ipv4
func ResolveIPv4(host string) (net.IP, error) {
	resolved := ResolveIPFromHosts(host, constant.AtypIPv4)
	if resolved != nil {
		return resolved, nil
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if !strings.Contains(host, ":") {
			return ip, nil
		}
		return nil, ErrIPVersion
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

	return nil, ErrIPNotFound
}

// ResolveIPv6 with a host, return ipv6
func ResolveIPv6(host string) (net.IP, error) {
	resolved := ResolveIPFromHosts(host, constant.AtypIPv6)
	if resolved != nil {
		return resolved, nil
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if strings.Contains(host, ":") {
			return ip, nil
		}
		return nil, ErrIPVersion
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

	return nil, ErrIPNotFound
}

// ResolveIP with a host, return ip
func ResolveIP(host string) (net.IP, error) {
	resolved := ResolveIPFromHosts(host, 0)
	if resolved != nil {
		return resolved, nil
	}

	if DefaultResolver != nil {
		return DefaultResolver.ResolveIP(host)
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
