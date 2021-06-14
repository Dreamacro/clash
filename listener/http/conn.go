package http

import (
	"io"
	"net"
)

type httpsConn struct {
	net.Conn

	reader io.Reader
}

func (c *httpsConn) Read(buf []byte) (int, error) {
	if c.reader != nil {
		n, err := c.reader.Read(buf)
		if err == nil {
			return n, err
		}

		c.reader = nil
	}

	return c.Conn.Read(buf)
}
