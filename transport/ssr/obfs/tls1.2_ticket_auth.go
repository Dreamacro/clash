package obfs

import (
	"bytes"
	"crypto/hmac"
	"encoding/binary"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/transport/ssr/tools"
)

func init() {
	register("tls1.2_ticket_auth", newTLS12Ticket, 5)
	register("tls1.2_ticket_fastauth", newTLS12Ticket, 5)
}

type tls12Ticket struct {
	*Base
	*authData
}

func newTLS12Ticket(b *Base) Obfs {
	r := &tls12Ticket{Base: b, authData: &authData{}}
	rand.Read(r.clientID[:])
	return r
}

type tls12TicketConn struct {
	net.Conn
	*tls12Ticket
	handshakeStatus int
	decoded         bytes.Buffer
	underDecoded    bytes.Buffer
	sendBuf         bytes.Buffer
}

func (t *tls12Ticket) StreamConn(c net.Conn) net.Conn {
	return &tls12TicketConn{Conn: c, tls12Ticket: t}
}

func (c *tls12TicketConn) Read(b []byte) (int, error) {
	if c.decoded.Len() > 0 {
		return c.decoded.Read(b)
	}

	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)
	n, err := c.Conn.Read(buf)
	if err != nil {
		return 0, err
	}

	if c.handshakeStatus == 8 {
		c.underDecoded.Write(buf[:n])
		for c.underDecoded.Len() > 5 {
			if !bytes.Equal(c.underDecoded.Bytes()[:3], []byte{0x17, 3, 3}) {
				c.underDecoded.Reset()
				return 0, errTLS12TicketAuthIncorrectMagicNumber
			}
			size := int(binary.BigEndian.Uint16(c.underDecoded.Bytes()[3:5]))
			if c.underDecoded.Len() < 5+size {
				break
			}
			c.underDecoded.Next(5)
			c.decoded.Write(c.underDecoded.Next(size))
		}
		n, _ = c.decoded.Read(b)
		return n, nil
	}

	if n < 11+32+1+32 {
		return 0, errTLS12TicketAuthTooShortData
	}

	if !hmac.Equal(buf[33:43], c.hmacSHA1(buf[11:33])[:10]) || !hmac.Equal(buf[n-10:n], c.hmacSHA1(buf[:n-10])[:10]) {
		return 0, errTLS12TicketAuthHMACError
	}

	c.Write(nil)
	return 0, nil
}

func (c *tls12TicketConn) Write(b []byte) (int, error) {
	length := len(b)
	if c.handshakeStatus == 8 {
		buf := tools.BufPool.Get().(*bytes.Buffer)
		defer tools.BufPool.Put(buf)
		defer buf.Reset()
		for len(b) > 2048 {
			size := rand.Intn(4096) + 100
			if len(b) < size {
				size = len(b)
			}
			packData(buf, b[:size])
			b = b[size:]
		}
		if len(b) > 0 {
			packData(buf, b)
		}
		_, err := c.Conn.Write(buf.Bytes())
		if err != nil {
			return 0, err
		}
		return length, nil
	}

	if len(b) > 0 {
		packData(&c.sendBuf, b)
	}

	if c.handshakeStatus == 0 {
		c.handshakeStatus = 1

		data := tools.BufPool.Get().(*bytes.Buffer)
		defer tools.BufPool.Put(data)
		defer data.Reset()

		data.Write([]byte{3, 3})
		c.packAuthData(data)
		data.WriteByte(0x20)
		data.Write(c.clientID[:])
		data.Write([]byte{0x00, 0x1c, 0xc0, 0x2b, 0xc0, 0x2f, 0xcc, 0xa9, 0xcc, 0xa8, 0xcc, 0x14, 0xcc, 0x13, 0xc0, 0x0a, 0xc0, 0x14, 0xc0, 0x09, 0xc0, 0x13, 0x00, 0x9c, 0x00, 0x35, 0x00, 0x2f, 0x00, 0x0a})
		data.Write([]byte{0x1, 0x0})

		ext := tools.BufPool.Get().(*bytes.Buffer)
		defer tools.BufPool.Put(ext)
		defer ext.Reset()

		host := c.getHost()
		ext.Write([]byte{0xff, 0x01, 0x00, 0x01, 0x00})
		packSNIData(ext, host)
		ext.Write([]byte{0, 0x17, 0, 0})
		c.packTicketBuf(ext, host)
		ext.Write([]byte{0x00, 0x0d, 0x00, 0x16, 0x00, 0x14, 0x06, 0x01, 0x06, 0x03, 0x05, 0x01, 0x05, 0x03, 0x04, 0x01, 0x04, 0x03, 0x03, 0x01, 0x03, 0x03, 0x02, 0x01, 0x02, 0x03})
		ext.Write([]byte{0x00, 0x05, 0x00, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00})
		ext.Write([]byte{0x00, 0x12, 0x00, 0x00})
		ext.Write([]byte{0x75, 0x50, 0x00, 0x00})
		ext.Write([]byte{0x00, 0x0b, 0x00, 0x02, 0x01, 0x00})
		ext.Write([]byte{0x00, 0x0a, 0x00, 0x06, 0x00, 0x04, 0x00, 0x17, 0x00, 0x18})

		binary.Write(data, binary.BigEndian, uint16(ext.Len()))
		data.ReadFrom(ext)

		ret := tools.BufPool.Get().(*bytes.Buffer)
		defer tools.BufPool.Put(ret)
		defer ret.Reset()

		ret.Write([]byte{0x16, 3, 1})
		binary.Write(ret, binary.BigEndian, uint16(data.Len()+4))
		ret.Write([]byte{1, 0})
		binary.Write(ret, binary.BigEndian, uint16(data.Len()))
		ret.ReadFrom(data)

		_, err := c.Conn.Write(ret.Bytes())
		if err != nil {
			return 0, err
		}
		return length, nil
	} else if c.handshakeStatus == 1 && len(b) == 0 {
		buf := tools.BufPool.Get().(*bytes.Buffer)
		defer tools.BufPool.Put(buf)
		defer buf.Reset()

		buf.Write([]byte{0x14, 3, 3, 0, 1, 1, 0x16, 3, 3, 0, 0x20})
		tools.AppendRandBytes(buf, 22)
		buf.Write(c.hmacSHA1(buf.Bytes())[:10])
		buf.ReadFrom(&c.sendBuf)

		c.handshakeStatus = 8

		_, err := c.Conn.Write(buf.Bytes())
		return 0, err
	}
	return length, nil
}

func packData(buf *bytes.Buffer, data []byte) {
	buf.Write([]byte{0x17, 3, 3})
	binary.Write(buf, binary.BigEndian, uint16(len(data)))
	buf.Write(data)
}

func (t *tls12Ticket) packAuthData(buf *bytes.Buffer) {
	binary.Write(buf, binary.BigEndian, uint32(time.Now().Unix()))
	tools.AppendRandBytes(buf, 18)
	buf.Write(t.hmacSHA1(buf.Bytes()[buf.Len()-22:])[:10])
}

func packSNIData(buf *bytes.Buffer, u string) {
	len := uint16(len(u))
	buf.Write([]byte{0, 0})
	binary.Write(buf, binary.BigEndian, len+5)
	binary.Write(buf, binary.BigEndian, len+3)
	buf.WriteByte(0)
	binary.Write(buf, binary.BigEndian, len)
	buf.WriteString(u)
}

func (c *tls12TicketConn) packTicketBuf(buf *bytes.Buffer, u string) {
	length := 16 * (rand.Intn(17) + 8)
	buf.Write([]byte{0, 0x23})
	binary.Write(buf, binary.BigEndian, uint16(length))
	tools.AppendRandBytes(buf, length)
}

func (t *tls12Ticket) hmacSHA1(data []byte) []byte {
	key := pool.Get(len(t.Key) + 32)
	defer pool.Put(key)
	copy(key, t.Key)
	copy(key[len(t.Key):], t.clientID[:])

	sha1Data := tools.HmacSHA1(key, data)
	return sha1Data[:10]
}

func (t *tls12Ticket) getHost() string {
	host := t.Param
	if len(host) == 0 {
		host = t.Host
	}
	if len(host) > 0 && host[len(host)-1] >= '0' && host[len(host)-1] <= '9' {
		host = ""
	}
	hosts := strings.Split(host, ",")
	host = hosts[rand.Intn(len(hosts))]
	return host
}
