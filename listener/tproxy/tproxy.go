package tproxy

import (
	"net"

	"github.com/Dreamacro/clash/adapter/inbound"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/socks5"
)

type Listener struct {
	listener net.Listener
	address  string
	closed   bool
}

func New(addr string, in chan<- C.ConnContext) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	tl := l.(*net.TCPListener)
	rc, err := tl.SyscallConn()
	if err != nil {
		return nil, err
	}

	err = setsockopt(rc, addr)
	if err != nil {
		return nil, err
	}

	rl := &Listener{
		listener: l,
		address:  addr,
	}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if rl.closed {
					break
				}
				continue
			}
			go rl.handleTProxy(c, in)
		}
	}()

	return rl, nil
}

func (l *Listener) Close() {
	l.closed = true
	l.listener.Close()
}

func (l *Listener) Address() string {
	return l.address
}

func (l *Listener) handleTProxy(conn net.Conn, in chan<- C.ConnContext) {
	target := socks5.ParseAddrToSocksAddr(conn.LocalAddr())
	conn.(*net.TCPConn).SetKeepAlive(true)
	in <- inbound.NewSocket(target, conn, C.TPROXY)
}
