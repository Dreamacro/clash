package snell

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/Dreamacro/go-shadowsocks2/shadowaead"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	CommandPing    byte = 0
	CommandConnect byte = 1

	CommandTunnel byte = 0
	CommandError  byte = 2

	Version byte = 1
)

var (
	bufferPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
)

type Snell struct {
	net.Conn
	buffer [1]byte
	reply  bool
}

func (s *Snell) Read(b []byte) (int, error) {
	if s.reply {
		return s.Conn.Read(b)
	}

	s.reply = true
	if _, err := io.ReadFull(s.Conn, s.buffer[:]); err != nil {
		return 0, err
	}

	if s.buffer[0] == CommandTunnel {
		return s.Conn.Read(b)
	} else if s.buffer[0] != CommandError {
		return 0, errors.New("Command not support")
	}

	// CommandError
	// 1 byte error code
	if _, err := io.ReadFull(s.Conn, s.buffer[:]); err != nil {
		return 0, err
	}
	errcode := int(s.buffer[0])

	// 1 byte error message length
	if _, err := io.ReadFull(s.Conn, s.buffer[:]); err != nil {
		return 0, err
	}
	length := int(s.buffer[0])
	msg := make([]byte, length)

	if _, err := io.ReadFull(s.Conn, msg); err != nil {
		return 0, err
	}

	return 0, fmt.Errorf("server reported code: %d, message: %s", errcode, string(msg))
}

func WriteHeader(conn net.Conn, host string, port uint) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)
	buf.WriteByte(Version)
	buf.WriteByte(CommandConnect)

	// clientID length & id
	buf.WriteByte(0)

	// host & port
	buf.WriteByte(uint8(len(host)))
	buf.WriteString(host)
	binary.Write(buf, binary.BigEndian, uint16(port))

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func StreamConn(conn net.Conn, psk []byte) net.Conn {
	cipher := &snellCipher{psk, chacha20poly1305.New}
	return &Snell{Conn: shadowaead.NewConn(conn, cipher)}
}
