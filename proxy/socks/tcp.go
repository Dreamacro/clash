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

type sockListener struct {
	net.Listener
	address string
	closed  bool
}

func NewSocksProxy(addr string) (*sockListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	sl := &sockListener{l, addr, false}
	go func() {
		log.Infof("SOCKS proxy listening at: %s", addr)
		for {
			c, err := l.Accept()
			if err != nil {
				if sl.closed {
					break
				}
				continue
			}
			go handleSocks(c)
		}
	}()

	return sl, nil
}

func (l *sockListener) Close() {
	l.closed = true
	l.Listener.Close()
}

func (l *sockListener) Address() string {
	return l.address
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
