package adapters

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
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
	UDP            bool   `proxy:"udp,omitempty"`
	SkipCertVerify bool   `proxy:"skip-cert-verify,omitempty"`
}

func (ss *Socks5) Dial(metadata *C.Metadata) (C.Conn, error) {
	c, err := net.DialTimeout("tcp", ss.addr, tcpTimeout)

	if err == nil && ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	if err != nil {
		return nil, fmt.Errorf("%s connect error", ss.addr)
	}
	tcpKeepAlive(c)
	var user *socks5.User
	if ss.user != "" {
		user = &socks5.User{
			Username: ss.user,
			Password: ss.pass,
		}
	}
	if err := socks5.ClientHandshake(c, serializesSocksAddr(metadata), socks5.CmdConnect, user); err != nil {
		return nil, err
	}
	return NewConn(c, ss), nil
}

func (ss *Socks5) DialUDP(metadata *C.Metadata) (net.PacketConn, net.Addr, error) {
	c, err := net.DialTimeout("tcp", ss.addr, tcpTimeout)

	if err == nil && ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	if err != nil {
		return nil, nil, fmt.Errorf("%s connect error", ss.addr)
	}
	tcpKeepAlive(c)
	var user *socks5.User
	if ss.user != "" {
		user = &socks5.User{
			Username: ss.user,
			Password: ss.pass,
		}
	}

	if err := socks5.ClientHandshake(c, serializesSocksAddr(metadata), socks5.CmdUDPAssociate, user); err != nil {
		return nil, nil, err
	}
	return &fakeUDPConn{Conn: c}, c.LocalAddr(), nil
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
			udp:  option.UDP,
		},
		addr:           net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
		user:           option.UserName,
		pass:           option.Password,
		tls:            option.TLS,
		skipCertVerify: option.SkipCertVerify,
		tlsConfig:      tlsConfig,
	}
}
