package http

import "net"

type listener struct {
	connections chan net.Conn
}

func (l *listener) Accept() (net.Conn, error) {
	conn, ok := <-l.connections
	if !ok {
		return nil, net.ErrClosed
	}

	return conn, nil
}

func (l *listener) Close() error {
	close(l.connections)

	return nil
}

func (l *listener) Addr() net.Addr {
	return nil
}

func (l *listener) Inject(conn net.Conn) {
	l.connections <- conn
}

func newListener() *listener {
	return &listener{connections: make(chan net.Conn, 255)}
}
