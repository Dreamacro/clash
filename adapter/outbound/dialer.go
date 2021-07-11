package outbound

import (
	"context"
	"net"
	"time"

	N "github.com/Dreamacro/clash/common/net"
	"github.com/Dreamacro/clash/component/dialer"
)

var KeyDialContext = struct{ string }{"key-dial-context"}

type DecorateFunc = func(conn net.Conn) (net.Conn, error)
type DialContextFunc = func(ctx context.Context, network, address string) (net.Conn, error)

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if dial := ctx.Value(KeyDialContext); dial != nil {
		return dial.(DialContextFunc)(ctx, network, address)
	}

	switch network {
	case "udp", "udp4", "udp6":
		addr, err := resolveUDPAddr(network, address)
		if err != nil {
			return nil, err
		}

		pc, err := dialer.ListenPacket(network, "")
		if err != nil {
			return nil, err
		}

		return N.NewStreamPacketConn(pc, addr), nil
	}

	c, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}

	if t, ok := c.(*net.TCPConn); ok {
		t.SetKeepAlive(true)
		t.SetKeepAlivePeriod(time.Second * 30)
	}

	return c, nil
}

func DialContextDecorated(
	ctx context.Context,
	network string,
	address string,
	decorator DecorateFunc,
) (net.Conn, error) {
	conn, err := DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}

	decorated, err := decorator(conn)
	if err != nil {
		conn.Close()

		return nil, err
	}

	return decorated, nil
}

func WithDialContext(parent context.Context, dialContext DialContextFunc) context.Context {
	if dialContext == nil {
		return context.WithValue(parent, KeyDialContext, nil)
	}

	return context.WithValue(parent, KeyDialContext, dialContext)
}
