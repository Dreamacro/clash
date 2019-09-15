package adapters

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
)

const (
	tcpTimeout = 5 * time.Second
)

var (
	globalClientSessionCache tls.ClientSessionCache
	once                     sync.Once
)

func urlToMetadata(rawURL string) (addr C.Metadata, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else if u.Scheme == "http" {
			port = "80"
		} else {
			err = fmt.Errorf("%s scheme not Support", rawURL)
			return
		}
	}

	addr = C.Metadata{
		AddrType: C.AtypDomainName,
		Host:     u.Hostname(),
		DstIP:    nil,
		DstPort:  port,
	}
	return
}

func tcpKeepAlive(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetKeepAlivePeriod(30 * time.Second)
	}
}

func getClientSessionCache() tls.ClientSessionCache {
	once.Do(func() {
		globalClientSessionCache = tls.NewLRUClientSessionCache(128)
	})
	return globalClientSessionCache
}

func serializesSocksAddr(metadata *C.Metadata) []byte {
	var buf [][]byte
	aType := uint8(metadata.AddrType)
	p, _ := strconv.Atoi(metadata.DstPort)
	port := []byte{uint8(p >> 8), uint8(p & 0xff)}
	switch metadata.AddrType {
	case socks5.AtypDomainName:
		len := uint8(len(metadata.Host))
		host := []byte(metadata.Host)
		buf = [][]byte{{aType, len}, host, port}
	case socks5.AtypIPv4:
		host := metadata.DstIP.To4()
		buf = [][]byte{{aType}, host, port}
	case socks5.AtypIPv6:
		host := metadata.DstIP.To16()
		buf = [][]byte{{aType}, host, port}
	}
	return bytes.Join(buf, nil)
}

func dialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{Timeout: timeout}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	returned := make(chan struct{})
	defer close(returned)

	type dialResult struct {
		net.Conn
		error
		ipv6 bool
		done bool
	}
	results := make(chan dialResult)
	var primary, fallback dialResult

	startRacer := func(ctx context.Context, host string, ipv6 bool) {
		var err error

		var ip net.IP
		if ipv6 {
			ip, err = dns.ResolveIPv6(host)
		} else {
			ip, err = dns.ResolveIPv4(host)
		}
		if err != nil {
			return
		}

		var c net.Conn
		if ipv6 {
			c, err = dialer.DialContext(ctx, "tcp6", net.JoinHostPort(ip.String(), port))
		} else {
			c, err = dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ip.String(), port))
		}
		if err != nil {
			return
		}

		select {
		case results <- dialResult{Conn: c, error: err, ipv6: ipv6}:
		case <-returned:
			if c != nil {
				c.Close()
			}
		}
	}

	go startRacer(ctx, host, false)
	go startRacer(ctx, host, true)

	for {
		select {
		case res := <-results:
			if res.error == nil {
				return res.Conn, nil
			}

			if res.ipv6 {
				primary = res
			} else {
				fallback = res
			}

			if primary.done && fallback.done {
				return nil, primary.error
			}
		}
	}
}

func resolveUDPAddr(network, address string) (*net.UDPAddr, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ip, err := dns.ResolveIP(host)
	if err != nil {
		return nil, err
	}
	return net.ResolveUDPAddr(network, net.JoinHostPort(ip.String(), port))
}
