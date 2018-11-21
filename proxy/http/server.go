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

type httpListener struct {
	net.Listener
	address string
	closed  bool
}

func NewHttpProxy(addr string) (*httpListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	hl := &httpListener{l, addr, false}

	go func() {
		log.Infoln("HTTP proxy listening at: %s", addr)
		for {
			c, err := hl.Accept()
			if err != nil {
				if hl.closed {
					break
				}
				continue
			}
			go handleConn(c)
		}
	}()

	return hl, nil
}

func (l *httpListener) Close() {
	l.Close()
	l.closed = false
}

func (l *httpListener) Address() string {
	return l.address
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
