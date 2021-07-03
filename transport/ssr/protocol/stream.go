package protocol

import (
	"bytes"
	"net"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/transport/ssr/tools"
)

type Conn struct {
	net.Conn
	Protocol
	decoded      bytes.Buffer
	underDecoded bytes.Buffer
}

func (c *Conn) Read(b []byte) (int, error) {
	if c.decoded.Len() > 0 {
		return c.decoded.Read(b)
	}

	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)
	n, err := c.Conn.Read(buf)
	if err != nil {
		return 0, err
	}
	c.underDecoded.Write(buf[:n])
	err = c.Decode(&c.decoded, &c.underDecoded)
	if err != nil {
		return 0, err
	}
	n, _ = c.decoded.Read(b)
	return n, nil
}

func (c *Conn) Write(b []byte) (int, error) {
	bLength := len(b)
	buf := tools.BufPool.Get().(*bytes.Buffer)
	defer tools.BufPool.Put(buf)
	defer buf.Reset()
	err := c.Encode(buf, b)
	if err != nil {
		return 0, err
	}
	_, err = c.Conn.Write(buf.Bytes())
	if err != nil {
		return 0, err
	}
	return bLength, nil
}
