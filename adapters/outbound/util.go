package adapters

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

const (
	tcpTimeout = 5 * time.Second
)

var (
	globalClientSessionCache tls.ClientSessionCache
	once                     sync.Once
)

// DelayTest get the delay for the specified URL
func DelayTest(proxy C.Proxy, url string) (t int16, err error) {
	addr, err := urlToMetadata(url)
	if err != nil {
		return
	}

	start := time.Now()
	instance, err := proxy.Generator(&addr)
	if err != nil {
		return
	}
	defer instance.Close()
	transport := &http.Transport{
		Dial: func(string, string) (net.Conn, error) {
			return instance.Conn(), nil
		},
		// from http.DefaultTransport
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := http.Client{Transport: transport}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	resp.Body.Close()
	t = int16(time.Since(start) / time.Millisecond)
	return
}

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
		IP:       nil,
		Port:     port,
	}
	return
}

func selectFast(in chan interface{}) chan interface{} {
	out := make(chan interface{})
	go func() {
		p, open := <-in
		if open {
			out <- p
		}
		close(out)
		for range in {
		}
	}()

	return out
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
