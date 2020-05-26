package dns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/Dreamacro/clash/component/dialer"

	D "github.com/miekg/dns"
)

type client struct {
	*D.Client
	r    *Resolver
	port string
	host string
}

func (c *client) Exchange(m *D.Msg) (msg *D.Msg, err error) {
	return c.ExchangeContext(context.Background(), m)
}

func (c *client) ExchangeContext(ctx context.Context, m *D.Msg) (msg *D.Msg, err error) {
	var ip net.IP
	if c.r == nil {
		// a default ip dns
		ip = net.ParseIP(c.host)
	} else {
		var err error
		if ip, err = c.r.ResolveIP(c.host); err != nil {
			return nil, fmt.Errorf("use default dns resolve failed: %w", err)
		}
	}

	d, err := dialer.Dialer()
	if err != nil {
		return nil, err
	}

	if dialer.DialHook != nil {
		network := "udp"
		if strings.HasPrefix(c.Client.Net, "tcp") {
			network = "tcp"
		}
		if err := dialer.DialHook(d, network, ip); err != nil {
			return nil, err
		}
	}

	c.Client.Dialer = d

	// miekg/dns ExchangeContext doesn't respond to context cancel.
	// this is a workaround
	type result struct {
		msg *D.Msg
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, _, err := c.Client.Exchange(m, net.JoinHostPort(ip.String(), c.port))
		ch <- result{msg, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ret := <-ch:
		return ret.msg, ret.err
	}
}
