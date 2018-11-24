package adapters

import (
	"encoding/json"
	"errors"
	"net"

	C "github.com/Dreamacro/clash/constant"
)

var errReject = errors.New("Reject this request")

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
	return nil, errReject
}

func (r *Reject) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": r.Type().String(),
	})
}

func NewReject() *Reject {
	return &Reject{}
}
