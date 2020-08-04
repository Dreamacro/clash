package dns

import (
	"bytes"
	"context"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"

	D "github.com/miekg/dns"
)

const (
	// dotMimeType is the DoH mimetype that should be used.
	dotMimeType = "application/dns-message"
)

type dohClient struct {
	url       string
	transport *http.Transport
	proxy *http.Transport
}

func (dc *dohClient) Exchange(m *D.Msg, proxy bool) (msg *D.Msg, err error) {
	return dc.ExchangeContext(context.Background(), m, proxy)
}

func (dc *dohClient) ExchangeContext(ctx context.Context, m *D.Msg, proxy bool) (msg *D.Msg, err error) {
	req, err := dc.newRequest(m)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	return dc.doRequest(req, proxy)
}

// newRequest returns a new DoH request given a dns.Msg.
func (dc *dohClient) newRequest(m *D.Msg) (*http.Request, error) {
	buf, err := m.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, dc.url, bytes.NewReader(buf))
	if err != nil {
		return req, err
	}

	req.Header.Set("content-type", dotMimeType)
	req.Header.Set("accept", dotMimeType)
	return req, nil
}

func (dc *dohClient) doRequest(req *http.Request, proxy bool) (msg *D.Msg, err error) {
	client := &http.Client{Transport: dc.transport}

	if proxy {
		client.Transport = dc.proxy
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	msg = &D.Msg{}
	err = msg.Unpack(buf)
	return msg, err
}

func newDoHClient(url string, r *Resolver) *dohClient {
	return &dohClient{
		url: url,
		transport: &http.Transport{
			TLSClientConfig:   &tls.Config{ClientSessionCache: globalSessionCache},
			ForceAttemptHTTP2: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}

				ip, err := r.ResolveIPv4(host)
				if err != nil {
					return nil, err
				}

				return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ip.String(), port))
			},
		},
		proxy: &http.Transport{
			TLSClientConfig:   &tls.Config{ClientSessionCache: globalSessionCache},
			ForceAttemptHTTP2: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}

				ip, err := r.ResolveIPv4(host)
				if err != nil {
					return nil, err
				}

				meta := C.Metadata{
					Host: ip.String(),
					DstIP: ip,
					DstPort: port,
					AddrType: C.AtypIPv4,
				}

				p := GetProxy()

				return p.DialContext(ctx, &meta)
			},
		},
	}
}
