package adapters

import (
	"io"
	"net"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

type Reject struct {
	*Base
}

func (r *Reject) Dial(metadata *C.Metadata) (C.Conn, error) {
	return newConn(&NopConn{}, r), nil
}

func NewReject() *Reject {
	return &Reject{
		Base: &Base{
			name: "REJECT",
			tp:   C.Reject,
		},
	}
}

type NopConn struct{}

func (rw *NopConn) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (rw *NopConn) Write(b []byte) (int, error) {
	return 0, io.EOF
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
