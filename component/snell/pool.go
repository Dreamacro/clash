package snell

import (
	"context"
	"net"
	"time"

	"github.com/Dreamacro/clash/component/pool"

	"github.com/Dreamacro/go-shadowsocks2/shadowaead"
)

type Pool struct {
	pool *pool.Pool
}

func (p *Pool) Get() (net.Conn, error) {
	return p.GetContext(context.Background())
}

func (p *Pool) GetContext(ctx context.Context) (net.Conn, error) {
	elm, err := p.pool.GetContext(ctx)
	if err != nil {
		return nil, err
	}

	return &PoolConn{elm.(*Snell), p}, nil
}

func (p *Pool) Put(conn net.Conn) {
	if err := HalfClose(conn); err != nil {
		conn.Close()
		return
	}

	p.pool.Put(conn)
}

type PoolConn struct {
	*Snell
	pool *Pool
}

func (pc *PoolConn) Read(b []byte) (int, error) {
	// save old status of reply (it mutable by Read)
	reply := pc.Snell.reply

	n, err := pc.Snell.Read(b)
	if err == shadowaead.ErrZeroChunk {
		// if reply is false, it should be client halfclose.
		// ignore error and read data again.
		if !reply {
			pc.Snell.reply = false
			return pc.Snell.Read(b)
		}
	}
	return n, err
}

func (pc *PoolConn) Write(b []byte) (int, error) {
	return pc.Snell.Write(b)
}

func (pc *PoolConn) Close() error {
	// clash use SetReadDeadline to break bidirectional copy between client and server.
	// reset it before reuse connection to avoid io timeout error.
	pc.Snell.Conn.SetReadDeadline(time.Time{})
	pc.pool.Put(pc.Snell)
	return nil
}

func NewPool(factory func(context.Context) (*Snell, error)) *Pool {
	p := pool.New(
		func(ctx context.Context) (interface{}, error) {
			return factory(ctx)
		},
		pool.WithAge(15000),
		pool.WithSize(10),
		pool.WithEvict(func(item interface{}) {
			item.(*Snell).Close()
		}),
	)

	return &Pool{p}
}
