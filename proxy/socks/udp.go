package socks

import (
	"net"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/tunnel"
)

var (
	_ = tunnel.NATInstance()
)

type SockUDPListener struct {
	net.PacketConn
	address string
	closed  bool
}

func NewSocksUDPProxy(addr string) (*SockUDPListener, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	sl := &SockUDPListener{l, addr, false}
	go func() {
		buf := pool.BufPool.Get().([]byte)
		defer pool.BufPool.Put(buf[:cap(buf)])
		for {
			n, remoteAddr, err := l.ReadFrom(buf)
			if err != nil {
				if sl.closed {
					break
				}
				continue
			}
			go handleSocksUDP(l, buf[:n], remoteAddr)
		}
	}()

	return sl, nil
}

func (l *SockUDPListener) Close() error {
	l.closed = true
	return l.PacketConn.Close()
}

func (l *SockUDPListener) Address() string {
	return l.address
}

func handleSocksUDP(c net.PacketConn, packet []byte, remoteAddr net.Addr) {
	target, payload, err := socks5.DecodeUDPPacket(packet)
	if err != nil {
		// Unresolved UDP packet, do nothing
		return
	}
	conn := newfakeConn(c, target.String(), remoteAddr, payload)
	tun.Add(adapters.NewSocket(target, conn, C.SOCKS, C.UDP))
}
