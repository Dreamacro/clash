package redir

import (
	"net"

	"github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel"
)

type TProxyListener struct {
	net.Listener
	address string
	closed  bool
}

func NewTProxy(addr string) (*TProxyListener, error) {
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

	rl := &TProxyListener{
		Listener: l,
		address:  addr,
	}

	go func() {
		log.Infoln("TProxy server listening at: %s", addr)
		for {
			c, err := l.Accept()
			if err != nil {
				if rl.closed {
					break
				}
				continue
			}
			go rl.handleRedir(c)
		}
	}()

	return rl, nil
}

func (l *TProxyListener) Close() {
	l.closed = true
	l.Listener.Close()
}

func (l *TProxyListener) Address() string {
	return l.address
}

func (l *TProxyListener) handleRedir(conn net.Conn) {
	target := socks5.ParseAddrToSocksAddr(conn.LocalAddr())
	conn.(*net.TCPConn).SetKeepAlive(true)
	tunnel.Add(inbound.NewSocket(target, conn, C.TPROXY))
}
