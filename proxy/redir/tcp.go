package redir

import (
	"net"

	"github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/tunnel"

	log "github.com/sirupsen/logrus"
)

var (
	tun = tunnel.Instance()
)

func NewRedirProxy(addr string) (chan<- struct{}, <-chan struct{}, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	done := make(chan struct{})
	closed := make(chan struct{})

	go func() {
		log.Infof("Redir proxy listening at: %s", addr)
		for {
			c, err := l.Accept()
			if err != nil {
				if _, open := <-done; !open {
					break
				}
				continue
			}
			go handleRedir(c)
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

func handleRedir(conn net.Conn) {
	target, err := parserPacket(conn)
	if err != nil {
		conn.Close()
		return
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	tun.Add(adapters.NewSocket(target, conn))
}
