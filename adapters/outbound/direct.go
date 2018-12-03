package adapters

import (
	"encoding/json"
	"net"

	C "github.com/Dreamacro/clash/constant"
)

// DirectAdapter is a directly connected adapter
type DirectAdapter struct {
	conn net.Conn
}

// Close is used to close connection
func (d *DirectAdapter) Close() {
	d.conn.Close()
}

// Conn is used to http request
func (d *DirectAdapter) Conn() net.Conn {
	return d.conn
}

type Direct struct{}

func (d *Direct) Name() string {
	return "DIRECT"
}

func (d *Direct) Type() C.AdapterType {
	return C.Direct
}

func (d *Direct) Generator(metadata *C.Metadata) (adapter C.ProxyAdapter, err error) {
	address := net.JoinHostPort(metadata.Host, metadata.Port)
	if metadata.IP != nil {
		address = net.JoinHostPort(metadata.IP.String(), metadata.Port)
	}

	c, err := net.DialTimeout("tcp", address, tcpTimeout)
	if err != nil {
		return
	}
	tcpKeepAlive(c)
	return &DirectAdapter{conn: c}, nil
}

func (d *Direct) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": d.Type().String(),
	})
}

func NewDirect() *Direct {
	return &Direct{}
}
