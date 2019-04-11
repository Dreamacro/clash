package adapters

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	C "github.com/Dreamacro/clash/constant"

	"github.com/Dreamacro/go-shadowsocks2/socks"
)

type Socks5 struct {
	*Base
	addr           string
	user           string
	pass           string
	tls            bool
	skipCertVerify bool
	tlsConfig      *tls.Config
}

type Socks5Option struct {
	Name           string `proxy:"name"`
	Server         string `proxy:"server"`
	Port           int    `proxy:"port"`
	UserName       string `proxy:"username,omitempty"`
	Password       string `proxy:"password,omitempty"`
	TLS            bool   `proxy:"tls,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
}

func (ss *Socks5) Dial(metadata *C.Metadata) (net.Conn, error) {
	c, err := dialTimeout("tcp", ss.addr, tcpTimeout)

	if err == nil && ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	if err != nil {
		return nil, fmt.Errorf("%s connect error", ss.addr)
	}
	tcpKeepAlive(c)
	if err := ss.shakeHand(metadata, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (ss *Socks5) shakeHand(metadata *C.Metadata, rw io.ReadWriter) error {
	buf := make([]byte, socks.MaxAddrLen)
	var err error

	// VER, NMETHODS, METHODS
	if len(ss.user) > 0 {
		_, err = rw.Write([]byte{5, 1, 2})
	} else {
		_, err = rw.Write([]byte{5, 1, 0})
	}
	if err != nil {
		return err
	}

	// VER, METHOD
	if _, err := io.ReadFull(rw, buf[:2]); err != nil {
		return err
	}

	if buf[0] != 5 {
		return errors.New("SOCKS version error")
	}

	if buf[1] == 2 {
		// password protocol version
		authMsg := &bytes.Buffer{}
		authMsg.WriteByte(1)
		authMsg.WriteByte(uint8(len(ss.user)))
		authMsg.WriteString(ss.user)
		authMsg.WriteByte(uint8(len(ss.pass)))
		authMsg.WriteString(ss.pass)

		if _, err := rw.Write(authMsg.Bytes()); err != nil {
			return err
		}

		if _, err := io.ReadFull(rw, buf[:2]); err != nil {
			return err
		}

		if buf[1] != 0 {
			return errors.New("rejected username/password")
		}
	} else if buf[1] != 0 {
		return errors.New("SOCKS need auth")
	}

	// VER, CMD, RSV, ADDR
	if _, err := rw.Write(bytes.Join([][]byte{{5, 1, 0}, serializesSocksAddr(metadata)}, []byte(""))); err != nil {
		return err
	}

	if _, err := io.ReadFull(rw, buf[:10]); err != nil {
		return err
	}

	return nil
}

func NewSocks5(option Socks5Option) *Socks5 {
	var tlsConfig *tls.Config
	if option.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: option.SkipCertVerify,
			ClientSessionCache: getClientSessionCache(),
			ServerName:         option.Server,
		}
	}

	return &Socks5{
		Base: &Base{
			name: option.Name,
			tp:   C.Socks5,
		},
		addr:           net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
		user:           option.UserName,
		pass:           option.Password,
		tls:            option.TLS,
		skipCertVerify: option.SkipCertVerify,
		tlsConfig:      tlsConfig,
	}
}
