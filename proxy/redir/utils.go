package redir

import (
	"net"

	"github.com/Dreamacro/clash/common/pool"
)

type packet struct {
	lAddr *net.UDPAddr
	buf   []byte
}

func (c *packet) Data() []byte {
	return c.buf
}

// WriteBack opens a new socket binding `addr` to wirte UDP packet back
func (c *packet) WriteBack(b []byte, addr net.Addr) (n int, err error) {
	tc, err := dialUDP("udp", addr.(*net.UDPAddr), c.lAddr)
	if err != nil {
		n = 0
		return
	}
	n, err = tc.Write(b)
	tc.Close()
	return
}

// LocalAddr returns the source IP/Port of UDP Packet
func (c *packet) LocalAddr() net.Addr {
	return c.lAddr
}

func (c *packet) Drop() {
	pool.Put(c.buf)
	return
}
