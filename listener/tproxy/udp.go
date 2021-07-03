package tproxy

import (
	"net"

	"github.com/Dreamacro/clash/adapter/inbound"
	"github.com/Dreamacro/clash/common/pool"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/socks5"
)

type UDPListener struct {
	packetConn net.PacketConn
	address    string
	closed     bool
}

func NewUDP(addr string, in chan<- *inbound.PacketAdapter) (*UDPListener, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	rl := &UDPListener{l, addr, false}

	c := l.(*net.UDPConn)

	rc, err := c.SyscallConn()
	if err != nil {
		return nil, err
	}

	err = setsockopt(rc, addr)
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
			handlePacketConn(l, in, buf[:n], lAddr, rAddr)
		}
	}()

	return rl, nil
}

func (l *UDPListener) Close() error {
	l.closed = true
	return l.packetConn.Close()
}

func (l *UDPListener) Address() string {
	return l.address
}

func handlePacketConn(pc net.PacketConn, in chan<- *inbound.PacketAdapter, buf []byte, lAddr *net.UDPAddr, rAddr *net.UDPAddr) {
	target := socks5.ParseAddrToSocksAddr(rAddr)
	pkt := &packet{
		lAddr: lAddr,
		buf:   buf,
	}
	select {
	case in <- inbound.NewPacket(target, pkt, C.TPROXY):
	default:
	}
}
