package adapters

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	P "github.com/Dreamacro/clash/common/picker"
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
		switch u.Scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		default:
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

func dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{}

	returned := make(chan struct{})
	defer close(returned)

	type dialResult struct {
		net.Conn
		error
		resolved bool
		ipv6     bool
		done     bool
	}
	results := make(chan dialResult)
	var primary, fallback dialResult

	startRacer := func(ctx context.Context, host string, ipv6 bool) {
		result := dialResult{ipv6: ipv6, done: true}
		defer func() {
			select {
			case results <- result:
			case <-returned:
				if result.Conn != nil {
					result.Conn.Close()
				}
			}
		}()

		var ip net.IP
		if ipv6 {
			ip, result.error = dns.ResolveIPv6(host)
		} else {
			ip, result.error = dns.ResolveIPv4(host)
		}
		if result.error != nil {
			return
		}
		result.resolved = true

		if ipv6 {
			result.Conn, result.error = dialer.DialContext(ctx, "tcp6", net.JoinHostPort(ip.String(), port))
		} else {
			result.Conn, result.error = dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ip.String(), port))
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

			if !res.ipv6 {
				primary = res
			} else {
				fallback = res
			}

			if primary.done && fallback.done {
				if primary.resolved {
					return nil, primary.error
				} else if fallback.resolved {
					return nil, fallback.error
				} else {
					return nil, primary.error
				}
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

// urlTest get the delay to the specified URL of the Proxy
func urlTest(ctx context.Context, p C.ProxyAdapter, url string) (t uint16, err error) {
	test := func() (t interface{}, err error) {
		addr, err := urlToMetadata(url)
		if err != nil {
			return
		}

		start := time.Now()
		instance, err := p.DialContext(ctx, &addr)
		if err != nil {
			return
		}
		defer instance.Close()

		req, err := http.NewRequest(http.MethodHead, url, nil)
		if err != nil {
			return
		}
		req = req.WithContext(ctx)

		transport := &http.Transport{
			Dial: func(string, string) (net.Conn, error) {
				return instance, nil
			},
			// from http.DefaultTransport
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		client := http.Client{Transport: transport}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
		t = uint16(time.Since(start) / time.Millisecond)
		return
	}
	select {
	case <-ctx.Done():
		return 0, errTimeout
	case result := <-healthCheckGroup.DoChan(getGroupKey(ctx), test):
		if result.Err == nil {
			return result.Val.(uint16), nil
		}
		return 0, result.Err
	}
}

func groupHealthCheck(ctx context.Context, proxies []C.Proxy, url string, checkAllInGroup bool,
	checkSingle func(ctx context.Context, proxy C.Proxy) (interface{}, error)) (interface{}, error) {
	var picker *P.Picker
	if checkAllInGroup {
		picker = P.WithoutAutoCancel(ctx)
	} else {
		picker, ctx = P.WithContext(ctx)
	}

	for _, p := range proxies {
		proxy := p
		picker.Go(func() (interface{}, error) {
			// since healthcheck of single proxy triggered in this method may get canceled by fast peer,
			// we should user a different group key(<group name><url><timeout><proxy name> used here)
			// to distinguish it from user-requested healthcheck(key <proxy name><url><timeout> used)
			newCtx := WithGroupKey(ctx, getGroupKey(ctx)+proxy.Name())
			return checkSingle(newCtx, proxy)
		})
	}

	result := picker.WaitWithoutCancel()
	if checkAllInGroup {
		picker.Wait()
	}

	if result == nil {
		return nil, errTimeout
	}
	return result, nil
}

type contextKeyType string

// MakeGroupKey makes a key for singleflight group of healthcheck
func MakeGroupKey(name, url string, timeout int64) string {
	return name + url + strconv.FormatInt(timeout, 10)
}

// WithGroupKey encapsulate a group key of healthCheckGroup into parent context
func WithGroupKey(parent context.Context, key string) context.Context {
	return context.WithValue(parent, contextKeyType("HealthCheckGroupKey"), key)
}

func getGroupKey(ctx context.Context) string {
	if key, ok := ctx.Value(contextKeyType("HealthCheckGroupKey")).(string); ok {
		return key
	}
	return ""
}
