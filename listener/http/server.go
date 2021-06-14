package http

import (
	"net"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	C "github.com/Dreamacro/clash/constant"
)

type Listener struct {
	listener net.Listener
	address  string
	closed   bool
	proxy    *Proxy
}

func New(addr string, in chan<- C.ConnContext) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	hl := &Listener{l, addr, false, NewProxy(in, cache.New(30*time.Second))}
	go func() {
		for {
			c, err := hl.listener.Accept()
			if err != nil {
				if hl.closed {
					break
				}
				continue
			}
			hl.proxy.ServeConn(c)
		}
	}()

	return hl, nil
}

func (l *Listener) Close() {
	l.closed = true
	l.listener.Close()
}

func (l *Listener) Address() string {
	return l.address
}
