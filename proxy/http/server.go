package http

import (
	"bufio"
	"net"
	"net/http"

	"github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel"
)

var (
	tun = tunnel.Instance()
)

func NewHttpProxy(addr string) (chan<- struct{}, <-chan struct{}, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	done := make(chan struct{})
	closed := make(chan struct{})

	go func() {
		log.Infoln("HTTP proxy listening at: %s", addr)
		for {
			c, err := l.Accept()
			if err != nil {
				if _, open := <-done; !open {
					break
				}
				continue
			}
			go handleConn(c)
		}
	}()

	go func() {
		<-done
		close(done)
		l.Close()
		closed <- struct{}{}
	}()

	return done, closed, nil
}

func handleConn(conn net.Conn) {
	br := bufio.NewReader(conn)
	request, err := http.ReadRequest(br)
	if err != nil {
		conn.Close()
		return
	}

	if request.Method == http.MethodConnect {
		_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			return
		}
		tun.Add(adapters.NewHTTPS(request, conn))
		return
	}

	tun.Add(adapters.NewHTTP(request, conn))
}
