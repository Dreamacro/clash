package adapters

import (
	"encoding/json"
	"io"
	"net"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

// RejectAdapter is a reject connected adapter
type RejectAdapter struct {
	conn net.Conn
}

// Close is used to close connection
func (r *RejectAdapter) Close() {}

// Conn is used to http request
func (r *RejectAdapter) Conn() net.Conn {
	return r.conn
}

type Reject struct {
}

func (r *Reject) Name() string {
	return "REJECT"
}

func (r *Reject) Type() C.AdapterType {
	return C.Reject
}

func (r *Reject) Generator(metadata *C.Metadata) (adapter C.ProxyAdapter, err error) {
	return &RejectAdapter{conn: &NopConn{}}, nil
}

func (r *Reject) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": r.Type().String(),
	})
}

func NewReject() *Reject {
	return &Reject{}
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
