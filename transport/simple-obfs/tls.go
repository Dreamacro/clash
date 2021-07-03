package obfs

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/Dreamacro/clash/common/pool"
)

func init() {
	rand.Seed(time.Now().Unix())
}

const (
	chunkSize = 1 << 14 // 2 ** 14 == 16 * 1024
)

// TLSObfs is shadowsocks tls simple-obfs implementation
type TLSObfs struct {
	net.Conn
	server        string
	remain        int
	firstRequest  bool
	firstResponse bool
}

func (to *TLSObfs) read(b []byte, discardN int) (int, error) {
	buf := pool.Get(discardN)
	_, err := io.ReadFull(to.Conn, buf)
	if err != nil {
		return 0, err
	}
	pool.Put(buf)

	sizeBuf := make([]byte, 2)
	_, err = io.ReadFull(to.Conn, sizeBuf)
	if err != nil {
		return 0, nil
	}

	length := int(binary.BigEndian.Uint16(sizeBuf))
	if length > len(b) {
		n, err := to.Conn.Read(b)
		if err != nil {
			return n, err
		}
		to.remain = length - n
		return n, nil
	}

	return io.ReadFull(to.Conn, b[:length])
}

func (to *TLSObfs) Read(b []byte) (int, error) {
	if to.remain > 0 {
		length := to.remain
		if length > len(b) {
			length = len(b)
		}

		n, err := io.ReadFull(to.Conn, b[:length])
		to.remain -= n
		return n, err
	}

	if to.firstResponse {
		// type + ver + lensize + 91 = 96
		// type + ver + lensize + 1 = 6
		// type + ver = 3
		to.firstResponse = false
		return to.read(b, 105)
	}

	// type + ver = 3
	return to.read(b, 3)
}
func (to *TLSObfs) Write(b []byte) (int, error) {
	length := len(b)
	for i := 0; i < length; i += chunkSize {
		end := i + chunkSize
		if end > length {
			end = length
		}

		n, err := to.write(b[i:end])
		if err != nil {
			return n, err
		}
	}
	return length, nil
}

func (to *TLSObfs) write(b []byte) (int, error) {
	if to.firstRequest {
		helloMsg := makeClientHelloMsg(b, to.server)
		_, err := to.Conn.Write(helloMsg)
		to.firstRequest = false
		return len(b), err
	}

	buf := &bytes.Buffer{}
	buf.Write([]byte{0x17, 0x03, 0x03})
	binary.Write(buf, binary.BigEndian, uint16(len(b)))
	buf.Write(b)
	_, err := to.Conn.Write(buf.Bytes())
	return len(b), err
}

// NewTLSObfs return a SimpleObfs
func NewTLSObfs(conn net.Conn, server string) net.Conn {
	return &TLSObfs{
		Conn:          conn,
		server:        server,
		firstRequest:  true,
		firstResponse: true,
	}
}

func makeClientHelloMsg(data []byte, server string) []byte {
	random := make([]byte, 28)
	sessionID := make([]byte, 32)
	rand.Read(random)
	rand.Read(sessionID)

	buf := &bytes.Buffer{}

	// handshake, TLS 1.0 version, length
	buf.WriteByte(22)
	buf.Write([]byte{0x03, 0x01})
	length := uint16(212 + len(data) + len(server))
	buf.WriteByte(byte(length >> 8))
	buf.WriteByte(byte(length & 0xff))

	// clientHello, length, TLS 1.2 version
	buf.WriteByte(1)
	buf.WriteByte(0)
	binary.Write(buf, binary.BigEndian, uint16(208+len(data)+len(server)))
	buf.Write([]byte{0x03, 0x03})

	// random with timestamp, sid len, sid
	binary.Write(buf, binary.BigEndian, uint32(time.Now().Unix()))
	buf.Write(random)
	buf.WriteByte(32)
	buf.Write(sessionID)

	// cipher suites
	buf.Write([]byte{0x00, 0x38})
	buf.Write([]byte{
		0xc0, 0x2c, 0xc0, 0x30, 0x00, 0x9f, 0xcc, 0xa9, 0xcc, 0xa8, 0xcc, 0xaa, 0xc0, 0x2b, 0xc0, 0x2f,
		0x00, 0x9e, 0xc0, 0x24, 0xc0, 0x28, 0x00, 0x6b, 0xc0, 0x23, 0xc0, 0x27, 0x00, 0x67, 0xc0, 0x0a,
		0xc0, 0x14, 0x00, 0x39, 0xc0, 0x09, 0xc0, 0x13, 0x00, 0x33, 0x00, 0x9d, 0x00, 0x9c, 0x00, 0x3d,
		0x00, 0x3c, 0x00, 0x35, 0x00, 0x2f, 0x00, 0xff,
	})

	// compression
	buf.Write([]byte{0x01, 0x00})

	// extension length
	binary.Write(buf, binary.BigEndian, uint16(79+len(data)+len(server)))

	// session ticket
	buf.Write([]byte{0x00, 0x23})
	binary.Write(buf, binary.BigEndian, uint16(len(data)))
	buf.Write(data)

	// server name
	buf.Write([]byte{0x00, 0x00})
	binary.Write(buf, binary.BigEndian, uint16(len(server)+5))
	binary.Write(buf, binary.BigEndian, uint16(len(server)+3))
	buf.WriteByte(0)
	binary.Write(buf, binary.BigEndian, uint16(len(server)))
	buf.Write([]byte(server))

	// ec_point
	buf.Write([]byte{0x00, 0x0b, 0x00, 0x04, 0x03, 0x01, 0x00, 0x02})

	// groups
	buf.Write([]byte{0x00, 0x0a, 0x00, 0x0a, 0x00, 0x08, 0x00, 0x1d, 0x00, 0x17, 0x00, 0x19, 0x00, 0x18})

	// signature
	buf.Write([]byte{
		0x00, 0x0d, 0x00, 0x20, 0x00, 0x1e, 0x06, 0x01, 0x06, 0x02, 0x06, 0x03, 0x05,
		0x01, 0x05, 0x02, 0x05, 0x03, 0x04, 0x01, 0x04, 0x02, 0x04, 0x03, 0x03, 0x01,
		0x03, 0x02, 0x03, 0x03, 0x02, 0x01, 0x02, 0x02, 0x02, 0x03,
	})

	// encrypt then mac
	buf.Write([]byte{0x00, 0x16, 0x00, 0x00})

	// extended master secret
	buf.Write([]byte{0x00, 0x17, 0x00, 0x00})

	return buf.Bytes()
}
