package redir

import (
	"net"

	"github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel"
)

type RedirListener struct {
	net.Listener
	address string
	tproxy  bool
	closed  bool
}

func NewRedirProxy(addr string, tproxyEnable bool) (*RedirListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	if tproxyEnable {
		tl := l.(*net.TCPListener)
		rc, err := tl.SyscallConn()
		if err != nil {
			return nil, err
		}

		err = setsockopt(rc, addr)
		if err != nil {
			return nil, err
		}

		log.Infoln("TProxy is enable")
	}

	rl := &RedirListener{Listener: l,
		address: addr,
		tproxy:  tproxyEnable,
	}

	go func() {
		log.Infoln("Redir proxy listening at: %s", addr)
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

func (l *RedirListener) Close() {
	l.closed = true
	l.Listener.Close()
}

func (l *RedirListener) Address() string {
	return l.address
}

func (l *RedirListener) handleRedir(conn net.Conn) {
	var target socks5.Addr
	var err error
	if l.tproxy {
		target = socks5.ParseAddrToSocksAddr(conn.LocalAddr())
	} else {
		target, err = parserPacket(conn)
		if err != nil {
			conn.Close()
			return
		}
	}

	conn.(*net.TCPConn).SetKeepAlive(true)
	tunnel.Add(inbound.NewSocket(target, conn, C.REDIR))
}
