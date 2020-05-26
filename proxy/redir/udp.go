package redir

import (
	"net"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/tunnel"
)

type RedirUDPListener struct {
	net.PacketConn
	address string
	closed  bool
}

func NewRedirUDPProxy(addr string) (*RedirUDPListener, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	rl := &RedirUDPListener{l, addr, false}

	c := l.(*net.UDPConn)

	err = setsockopt(c, addr)
	if err != nil {
		return nil, err
	}

	go func() {
		oob := make([]byte, 1024)
		for {
			buf := pool.Get(pool.RelayBufferSize)
			n, oobn, _, lAddr, err := c.ReadMsgUDP(buf, oob)
			if err != nil {
				pool.Put(buf)
				if rl.closed {
					break
				}
				continue
			}

			rAddr, err := getOrigDst(oob, oobn)
			if err != nil {
				continue
			}
			handleRedirUDP(l, buf[:n], lAddr, rAddr)
		}
	}()

	return rl, nil
}

func (l *RedirUDPListener) Close() error {
	l.closed = true
	return l.PacketConn.Close()
}

func (l *RedirUDPListener) Address() string {
	return l.address
}

func handleRedirUDP(pc net.PacketConn, buf []byte, lAddr *net.UDPAddr, rAddr *net.UDPAddr) {
	target := socks5.ParseAddrToSocksAddr(rAddr)
	pkt := &packet{
		lAddr: lAddr,
		buf:   buf,
	}
	tunnel.AddPacket(adapters.NewPacket(target, pkt, C.REDIR))
}
