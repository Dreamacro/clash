package outbound

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

const (
	DropDuration = 60 * time.Second
) 

type Reject struct {
	*Base
	duration time.Duration
}

// DialContext implements C.ProxyAdapter
func (r *Reject) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	return NewConn(&NopConn{
		duration: r.duration,
	}, r), nil
}

// DialUDP implements C.ProxyAdapter
func (r *Reject) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	return nil, errors.New("match reject rule")
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

func NewRejectDrop() *Reject {
	return &Reject{
		Base: &Base{
			name: "REJECT-DROP",
			tp:   C.RejectDrop,
			udp:  true,
		},
		duration: DropDuration,
	}
}

type NopConn struct{
	duration time.Duration
}

func (rw *NopConn) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (rw *NopConn) Write(b []byte) (int, error) {
	<- time.After(rw.duration)
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
