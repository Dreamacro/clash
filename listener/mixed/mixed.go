package mixed

import (
	"net"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	N "github.com/Dreamacro/clash/common/net"
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
}

func New(addr string, in chan<- C.ConnContext) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	ml := &Listener{l, addr, false, cache.New(30 * time.Second)}
	go func() {
		for {
			c, err := ml.listener.Accept()
			if err != nil {
				if ml.closed {
					break
				}
				continue
			}
			go handleConn(c, in, ml.cache)
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

func handleConn(conn net.Conn, in chan<- C.ConnContext, cache *cache.Cache) {
	bufConn := N.NewBufferedConn(conn)
	head, err := bufConn.Peek(1)
	if err != nil {
		return
	}

	if head[0] == socks5.Version {
		socks.HandleSocks(bufConn, in)
		return
	}

	http.HandleConn(bufConn, in, cache)
}
