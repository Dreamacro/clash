package mixed

import (
	"net"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/listener/http"
	"github.com/Dreamacro/clash/listener/socks"
	"github.com/Dreamacro/clash/transport/socks5"
)

type Listener struct {
	listener net.Listener
	address  string
	closed   bool
	cache    *cache.Cache
	http     *http.Proxy
}

func New(addr string, in chan<- C.ConnContext) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := cache.New(30 * time.Second)
	ml := &Listener{l, addr, false, c, http.NewProxy(in, c)}
	go func() {
		for {
			c, err := ml.listener.Accept()
			if err != nil {
				if ml.closed {
					break
				}
				continue
			}
			go ml.handleConn(c, in)
		}
	}()

	return ml, nil
}

func (l *Listener) Close() {
	l.closed = true
	l.listener.Close()
}

func (l *Listener) Address() string {
	return l.address
}

func (l *Listener) handleConn(conn net.Conn, in chan<- C.ConnContext) {
	bufConn := NewBufferedConn(conn)
	head, err := bufConn.Peek(1)
	if err != nil {
		return
	}

	if head[0] == socks5.Version {
		socks.HandleSocks(bufConn, in)
		return
	}

	l.http.ServeConn(bufConn)
}
