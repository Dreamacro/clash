package dialer

import (
	"context"
	"errors"
	"net"

	"github.com/Dreamacro/clash/component/resolver"
)

func Dialer() (*net.Dialer, error) {
	dialer := &net.Dialer{}
	if DialerHook != nil {
		if err := DialerHook(dialer); err != nil {
			return nil, err
		}
	}

	return dialer, nil
}

func ListenConfig() (*net.ListenConfig, error) {
	cfg := &net.ListenConfig{}
	if ListenConfigHook != nil {
		if err := ListenConfigHook(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func Dial(network, address string) (net.Conn, error) {
	return DialContext(context.Background(), network, address)
}

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp4", "tcp6", "udp4", "udp6":
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		dialer, err := Dialer()
		if err != nil {
			return nil, err
		}

		var ip net.IP
		switch network {
		case "tcp4", "udp4":
			ip, err = resolver.ResolveIPv4(host)
		default:
			ip, err = resolver.ResolveIPv6(host)
		}

		if err != nil {
			return nil, err
		}

		if DialHook != nil {
			if err := DialHook(dialer, network, ip); err != nil {
				return nil, err
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	case "tcp", "udp":
		return dualStackDialContext(ctx, network, address)
	default:
		return nil, errors.New("network invalid")
	}
}

func ListenPacket(network, address string) (net.PacketConn, error) {
	lc, err := ListenConfig()
	if err != nil {
		return nil, err
	}

	if ListenPacketHook != nil && address == "" {
		ip, err := ListenPacketHook()
		if err != nil {
			return nil, err
		}
		address = net.JoinHostPort(ip.String(), "0")
	}
	return lc.ListenPacket(context.Background(), network, address)
}

func dualStackDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

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

	startRacer := func(ctx context.Context, network, host string, ipv6 bool) {
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

		dialer, err := Dialer()
		if err != nil {
			result.error = err
			return
		}

		var ip net.IP
		if ipv6 {
			ip, result.error = resolver.ResolveIPv6(host)
		} else {
			ip, result.error = resolver.ResolveIPv4(host)
		}
		if result.error != nil {
			return
		}
		result.resolved = true

		if DialHook != nil {
			if result.error = DialHook(dialer, network, ip); result.error != nil {
				return
			}
		}
		result.Conn, result.error = dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	go startRacer(ctx, network+"4", host, false)
	go startRacer(ctx, network+"6", host, true)

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
