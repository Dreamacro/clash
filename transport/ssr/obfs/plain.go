package obfs

import "net"

type plain struct{}

func init() {
	register("plain", newPlain, 0)
}

func newPlain(b *Base) Obfs {
	return &plain{}
}

func (p *plain) StreamConn(c net.Conn) net.Conn { return c }
