package vless

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net"

	"github.com/Dreamacro/clash/component/vmess"
	"github.com/gofrs/uuid"
)

type Conn struct {
	net.Conn
	dst *vmess.DstAddr
	id  *uuid.UUID

	received bool
}

func (vc *Conn) Read(b []byte) (int, error) {
	if vc.received {
		return vc.Conn.Read(b)
	}

	if err := vc.recvResponse(); err != nil {
		return 0, err
	}
	vc.received = true
	return vc.Conn.Read(b)
}

func (vc *Conn) sendRequest() error {
	buf := &bytes.Buffer{}

	buf.WriteByte(Version)   // protocol version
	buf.Write(vc.id.Bytes()) // 16 bytes of uuid
	buf.WriteByte(0)         // addon data length. 0 means no addon data

	// command
	if vc.dst.UDP {
		buf.WriteByte(vmess.CommandUDP)
	} else {
		buf.WriteByte(vmess.CommandTCP)
	}

	// Port AddrType Addr
	binary.Write(buf, binary.BigEndian, uint16(vc.dst.Port))
	buf.WriteByte(vc.dst.AddrType)
	buf.Write(vc.dst.Addr)

	_, err := vc.Conn.Write(buf.Bytes())
	return err
}

func (vc *Conn) recvResponse() error {
	var err error
	buf := make([]byte, 1)
	_, err = io.ReadFull(vc.Conn, buf)
	if err != nil {
		return err
	}

	if buf[0] != Version {
		return errors.New("unexpected response version")
	}

	_, err = io.ReadFull(vc.Conn, buf)
	if err != nil {
		return err
	}

	length := int64(buf[0])
	if length != 0 { // addon data length > 0
		io.CopyN(ioutil.Discard, vc.Conn, length) // just discard
	}

	return nil
}

// newConn return a Conn instance
func newConn(conn net.Conn, id *uuid.UUID, dst *vmess.DstAddr) (*Conn, error) {
	c := &Conn{
		Conn: conn,
		id:   id,
		dst:  dst,
	}
	if err := c.sendRequest(); err != nil {
		return nil, err
	}
	return c, nil
}
