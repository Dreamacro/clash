package mixed

import (
	"bufio"
	"net"
)

type ConnEx struct {
	r *bufio.Reader
	net.Conn
}

func NewConnEx(c net.Conn) *ConnEx {
	return &ConnEx{bufio.NewReader(c), c}
}

// Peek returns the next n bytes without advancing the reader.
func (c *ConnEx) Peek(n int) ([]byte, error) {
	return c.r.Peek(n)
}

func (c *ConnEx) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

// Reader returns the internal bufio.Reader.
func (c *ConnEx) Reader() *bufio.Reader {
	return c.r
}
