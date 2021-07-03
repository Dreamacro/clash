package redir

import (
	"net"

	"github.com/Dreamacro/clash/adapter/inbound"
	C "github.com/Dreamacro/clash/constant"
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
	rl := &Listener{l, addr, false}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if rl.closed {
					break
				}
				continue
			}
			go handleRedir(c, in)
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

func handleRedir(conn net.Conn, in chan<- C.ConnContext) {
	target, err := parserPacket(conn)
	if err != nil {
		conn.Close()
		return
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	in <- inbound.NewSocket(target, conn, C.REDIR)
}
