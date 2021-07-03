package outbound

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/socks5"
)

type Socks5 struct {
	*Base
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

// StreamConn implements C.ProxyAdapter
func (ss *Socks5) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	if ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err := cc.Handshake()
		c = cc
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", ss.addr, err)
		}
	}

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

// DialContext implements C.ProxyAdapter
func (ss *Socks5) DialContext(ctx context.Context, metadata *C.Metadata) (_ C.Conn, err error) {
	c, err := dialer.DialContext(ctx, "tcp", ss.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", ss.addr, err)
	}
	tcpKeepAlive(c)

	defer safeConnClose(c, err)

	c, err = ss.StreamConn(c, metadata)
	if err != nil {
		return nil, err
	}

	return NewConn(c, ss), nil
}

// DialUDP implements C.ProxyAdapter
func (ss *Socks5) DialUDP(metadata *C.Metadata) (_ C.PacketConn, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), C.DefaultTCPTimeout)
	defer cancel()
	c, err := dialer.DialContext(ctx, "tcp", ss.addr)
	if err != nil {
		err = fmt.Errorf("%s connect error: %w", ss.addr, err)
		return
	}

	if ss.tls {
		cc := tls.Client(c, ss.tlsConfig)
		err = cc.Handshake()
		c = cc
	}

	defer safeConnClose(c, err)

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
		err = fmt.Errorf("client hanshake error: %w", err)
		return
	}

	pc, err := dialer.ListenPacket("udp", "")
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

	// Support unspecified UDP bind address.
	bindUDPAddr := bindAddr.UDPAddr()
	if bindUDPAddr == nil {
		err = errors.New("invalid UDP bind address")
		return
	} else if bindUDPAddr.IP.IsUnspecified() {
		serverAddr, err := resolveUDPAddr("udp", ss.Addr())
		if err != nil {
			return nil, err
		}

		bindUDPAddr.IP = serverAddr.IP
	}

	return newPacketConn(&socksPacketConn{PacketConn: pc, rAddr: bindUDPAddr, tcpConn: c}, ss), nil
}

func NewSocks5(option Socks5Option) *Socks5 {
	var tlsConfig *tls.Config
	if option.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: option.SkipCertVerify,
			ServerName:         option.Server,
		}
	}

	return &Socks5{
		Base: &Base{
			name: option.Name,
			addr: net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
			tp:   C.Socks5,
			udp:  option.UDP,
		},
		user:           option.UserName,
		pass:           option.Password,
		tls:            option.TLS,
		skipCertVerify: option.SkipCertVerify,
		tlsConfig:      tlsConfig,
	}
}

type socksPacketConn struct {
	net.PacketConn
	rAddr   net.Addr
	tcpConn net.Conn
}

func (uc *socksPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	packet, err := socks5.EncodeUDPPacket(socks5.ParseAddrToSocksAddr(addr), b)
	if err != nil {
		return
	}
	return uc.PacketConn.WriteTo(packet, uc.rAddr)
}

func (uc *socksPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, _, e := uc.PacketConn.ReadFrom(b)
	if e != nil {
		return 0, nil, e
	}
	addr, payload, err := socks5.DecodeUDPPacket(b)
	if err != nil {
		return 0, nil, err
	}

	udpAddr := addr.UDPAddr()
	if udpAddr == nil {
		return 0, nil, errors.New("parse udp addr error")
	}

	// due to DecodeUDPPacket is mutable, record addr length
	copy(b, payload)
	return n - len(addr) - 3, udpAddr, nil
}

func (uc *socksPacketConn) Close() error {
	uc.tcpConn.Close()
	return uc.PacketConn.Close()
}
