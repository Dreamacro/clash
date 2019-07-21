package adapters

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/common/pool"
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
	return c, nil
}

func (ss *Socks5) DialUDP(metadata *C.Metadata) (net.PacketConn, net.Addr, error) {
	c, err := dialTimeout("tcp", ss.addr, tcpTimeout)

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

	bindAddr, err := socks5.ClientHandshake(c, serializesSocksAddr(metadata), socks5.CmdUDPAssociate, user)
	if err != nil {
		return nil, nil, fmt.Errorf("%v client hanshake error", err)
	}

	addr, err := net.ResolveUDPAddr("udp", bindAddr.String())
	if err != nil {
		return nil, nil, err
	}

	targetAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(metadata.String(), metadata.DstPort))
	if err != nil {
		return nil, nil, err
	}

	go func() {
		io.Copy(ioutil.Discard, c)
		c.Close()
	}()

	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, nil, err
	}

	return &socksUDPConn{PacketConn: pc, rAddr: targetAddr}, addr, nil
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
	rAddr net.Addr
}

func (uc *socksUDPConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	buf := pool.BufPool.Get().([]byte)
	defer pool.BufPool.Put(buf[:cap(buf)])
	buffer, err := socks5.EncodeUDPPacket(uc.rAddr.String(), b)
	if err != nil {
		return
	}
	n, _ = buffer.Read(buf)
	return uc.PacketConn.WriteTo(buf[:n], addr)
}

func (uc *socksUDPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, a, e := uc.PacketConn.ReadFrom(b)
	rAddr, err := socks5.DecodeUDPPacket(b)
	if err != nil {
		return 0, nil, err
	}
	copy(b, b[3+len(rAddr):])
	return n - len(rAddr) - 3, a, e
}
