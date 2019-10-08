package adapters

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
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
	var user *socks5.User
	if ss.user != "" {
		user = &socks5.User{
			Username: ss.user,
			Password: ss.pass,
		}
	}
	if _, err := socks5.ClientHandshake(c, serializesSocksAddr(metadata), socks5.CmdConnect, user); err != nil {
		return nil, err
	}
	return newConn(c, ss), nil
}

func (ss *Socks5) DialUDP(metadata *C.Metadata) (_ C.PacketConn, _ net.Addr, err error) {
	c, err := dialTimeout("tcp", ss.addr, tcpTimeout)
	if err != nil {
		err = fmt.Errorf("%s connect error", ss.addr)
		return
	}

	if ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	tcpKeepAlive(c)
	var user *socks5.User
	if ss.user != "" {
		user = &socks5.User{
			Username: ss.user,
			Password: ss.pass,
		}
	}

	bindAddr, err := socks5.ClientHandshake(c, serializesSocksAddr(metadata), socks5.CmdUDPAssociate, user)
	if err != nil {
		err = fmt.Errorf("%v client hanshake error", err)
		return
	}

	addr, err := net.ResolveUDPAddr("udp", bindAddr.String())
	if err != nil {
		return
	}

	targetAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(metadata.String(), metadata.DstPort))
	if err != nil {
		return
	}

	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return
	}

	go func() {
		io.Copy(ioutil.Discard, c)
		c.Close()
		// A UDP association terminates when the TCP connection that the UDP
		// ASSOCIATE request arrived on terminates. RFC1928
		pc.Close()
	}()

	return newPacketConn(&socksUDPConn{PacketConn: pc, rAddr: targetAddr, tcpConn: c}, ss), addr, nil
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

type socksUDPConn struct {
	net.PacketConn
	rAddr   net.Addr
	tcpConn net.Conn
}

func (uc *socksUDPConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	packet, err := socks5.EncodeUDPPacket(uc.rAddr.String(), b)
	if err != nil {
		return
	}
	return uc.PacketConn.WriteTo(packet, addr)
}

func (uc *socksUDPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, a, e := uc.PacketConn.ReadFrom(b)
	addr, payload, err := socks5.DecodeUDPPacket(b)
	if err != nil {
		return 0, nil, err
	}
	copy(b, payload)
	return n - len(addr) - 3, a, e
}

func (uc *socksUDPConn) Close() error {
	uc.tcpConn.Close()
	return uc.PacketConn.Close()
}
