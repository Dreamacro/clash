package outbound

import (
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type RouteHopAddr struct {
	net.Addr
	name string
}

type routeConn struct {
	net.Conn
	name string
}

type routePacketConn struct {
	net.PacketConn
	stream net.Conn
	name   string
}

func (addr *RouteHopAddr) Hops() C.Hops {
	if addr.Addr != nil {
		if r, ok := addr.Addr.(C.Route); ok {
			return append(r.Hops(), addr.name)
		}
	}

	return append(make(C.Hops, 0, 32), addr.name)
}

func (c *routeConn) LocalAddr() net.Addr {
	return &RouteHopAddr{
		Addr: c.Conn.LocalAddr(),
		name: c.name,
	}
}

func (c *routePacketConn) RemoteAddr() net.Addr {
	return c.stream.RemoteAddr()
}

func (c *routePacketConn) LocalAddr() net.Addr {
	return &RouteHopAddr{
		Addr: c.PacketConn.LocalAddr(),
		name: c.name,
	}
}

func (c *routePacketConn) Read(b []byte) (n int, err error) {
	return c.stream.Read(b)
}

func (c *routePacketConn) Write(b []byte) (n int, err error) {
	return c.stream.Write(b)
}

func WithRouteHop(c net.Conn, p C.ProxyAdapter) net.Conn {
	if pc, ok := c.(net.PacketConn); ok {
		return &routePacketConn{
			PacketConn: pc,
			stream:     c,
			name:       p.Name(),
		}
	}

	return &routeConn{
		Conn: c,
		name: p.Name(),
	}
}
