package outbound

import (
	"context"
	"net"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

type Reject struct {
	*Base
}

// DialContext implements C.ProxyAdapter
func (r *Reject) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return WithRouteHop(&NopConn{}, r), nil
}

func NewReject() *Reject {
	return &Reject{
		Base: &Base{
			name: "REJECT",
			tp:   C.Reject,
			udp:  true,
		},
	}
}

type NopConn struct{}

func (rw *NopConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return 0, nil, net.ErrClosed
}

func (rw *NopConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return 0, net.ErrClosed
}

func (rw *NopConn) Read(b []byte) (int, error) {
	return 0, net.ErrClosed
}

func (rw *NopConn) Write(b []byte) (int, error) {
	return 0, net.ErrClosed
}

// Close is fake function for net.Conn
func (rw *NopConn) Close() error { return nil }

// LocalAddr is fake function for net.Conn
func (rw *NopConn) LocalAddr() net.Addr { return nil }

// RemoteAddr is fake function for net.Conn
func (rw *NopConn) RemoteAddr() net.Addr { return nil }

// SetDeadline is fake function for net.Conn
func (rw *NopConn) SetDeadline(time.Time) error { return nil }

// SetReadDeadline is fake function for net.Conn
func (rw *NopConn) SetReadDeadline(time.Time) error { return nil }

// SetWriteDeadline is fake function for net.Conn
func (rw *NopConn) SetWriteDeadline(time.Time) error { return nil }
