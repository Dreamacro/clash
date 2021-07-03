package protocol

import (
	"bytes"
	"net"
)

type origin struct{}

func init() { register("origin", newOrigin, 0) }

func newOrigin(b *Base) Protocol { return &origin{} }

func (o *origin) StreamConn(c net.Conn, iv []byte) net.Conn { return c }

func (o *origin) PacketConn(c net.PacketConn) net.PacketConn { return c }

func (o *origin) Decode(dst, src *bytes.Buffer) error {
	dst.ReadFrom(src)
	return nil
}

func (o *origin) Encode(buf *bytes.Buffer, b []byte) error {
	buf.Write(b)
	return nil
}

func (o *origin) DecodePacket(b []byte) ([]byte, error) { return b, nil }

func (o *origin) EncodePacket(buf *bytes.Buffer, b []byte) error {
	buf.Write(b)
	return nil
}
