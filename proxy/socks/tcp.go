package socks

import (
	"net"

	"github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/tunnel"

	"github.com/Dreamacro/go-shadowsocks2/socks"
	log "github.com/sirupsen/logrus"
)

var (
	tun = tunnel.Instance()
)

func NewSocksProxy(addr string) (chan<- struct{}, <-chan struct{}, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	done := make(chan struct{})
	closed := make(chan struct{})

	go func() {
		log.Infof("SOCKS proxy listening at: %s", addr)
		for {
			c, err := l.Accept()
			if err != nil {
				if _, open := <-done; !open {
					break
				}
				continue
			}
			go handleSocks(c)
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

func handleSocks(conn net.Conn) {
	target, err := socks.Handshake(conn)
	if err != nil {
		conn.Close()
		return
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	tun.Add(adapters.NewSocket(target, conn))
}
