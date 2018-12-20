package obfs

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net"
)

var HTTP = "GET / HTTP/1.1\r\n" +
	"Host: %s\r\n" +
	"User-Agent: curl/7.%d.%d\r\n" +
	"Upgrade: websocket\r\n" +
	"Connection: Upgrade\r\n" +
	"Sec-WebSocket-Key: %s\r\n" +
	"Content-Length: %d\r\n" +
	"\r\n"

// HTTPObfs is shadowsocks http simple-obfs implementation
type HTTPObfs struct {
	net.Conn
	host          string
	port          string
	buf           []byte
	offset        int
	firstRequest  bool
	firstResponse bool
}

func (ho *HTTPObfs) Read(b []byte) (int, error) {
	if ho.buf != nil {
		n := copy(b, ho.buf[ho.offset:])
		ho.offset += n
		if ho.offset == len(ho.buf) {
			ho.buf = nil
		}
		return n, nil
	}

	if ho.firstResponse {
		buf := bufPool.Get().([]byte)
		n, err := ho.Conn.Read(buf)
		if err != nil {
			bufPool.Put(buf[:cap(buf)])
			return 0, err
		}
		idx := bytes.Index(buf[:n], []byte("\r\n\r\n"))
		if idx == -1 {
			bufPool.Put(buf[:cap(buf)])
			return 0, io.EOF
		}
		ho.firstResponse = false
		length := n - (idx + 4)
		n = copy(b, buf[idx+4:n])
		if length > n {
			ho.buf = buf[:idx+4+length]
			ho.offset = idx + 4 + n
		} else {
			bufPool.Put(buf[:cap(buf)])
		}
		return n, nil
	}
	return ho.Conn.Read(b)
}

func (ho *HTTPObfs) Write(b []byte) (int, error) {
	if ho.firstRequest {
		randBytes := make([]byte, 16)
		rand.Read(randBytes)
		buf := new(bytes.Buffer)
		buf.WriteString(
			fmt.Sprintf(HTTP,
				ho.host,
				rand.Int()%54,
				rand.Int()%2,
				base64.URLEncoding.EncodeToString(randBytes),
				len(b)))

		buf.Write(b)
		ho.firstRequest = false
		return ho.Conn.Write(buf.Bytes())

	}
	return ho.Conn.Write(b)
}

// NewHTTPObfs return a HTTPObfs
func NewHTTPObfs(conn net.Conn, host string, port string) net.Conn {
	return &HTTPObfs{
		Conn:          conn,
		firstRequest:  true,
		firstResponse: true,
		host:          host,
		port:          port,
	}
}
