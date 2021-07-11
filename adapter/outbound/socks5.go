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

	N "github.com/Dreamacro/clash/common/net"
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

// DialContext implements C.ProxyAdapter
func (ss *Socks5) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "udp", "udp4", "udp6":
		ctx, cancel := context.WithTimeout(ctx, C.DefaultTCPTimeout)
		defer cancel()

		addr, err := resolveUDPAddr(network, address)
		if err != nil {
			return nil, err
		}

		control, err := DialContext(ctx, "tcp", ss.addr)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", ss.addr, err)
		}

		defer safeConnClose(control, err)

		if ss.tls {
			cc := tls.Client(control, ss.tlsConfig)
			err = cc.Handshake()
			if err != nil {
				return nil, err
			}
			control = cc
		}

		var user *socks5.User
		if ss.user != "" {
			user = &socks5.User{
				Username: ss.user,
				Password: ss.pass,
			}
		}

		bindAddr, err := socks5.ClientHandshake(control, socks5.ParseAddrToSocksAddr(addr), socks5.CmdUDPAssociate, user)
		if err != nil {
			err = fmt.Errorf("client hanshake error: %w", err)
			return nil, err
		}

		// Support unspecified UDP bind address.
		bindUDPAddr := bindAddr.UDPAddr()
		if bindUDPAddr == nil {
			err = errors.New("invalid UDP bind address")
			return nil, err
		} else if bindUDPAddr.IP.IsUnspecified() {
			serverAddr, err := resolveUDPAddr("udp", ss.Addr())
			if err != nil {
				return nil, err
			}

			bindUDPAddr.IP = serverAddr.IP
		}

		conn, err := DialContextDecorated(ctx, network, bindUDPAddr.String(), func(conn net.Conn) (net.Conn, error) {
			pc := &socksPacketConn{Conn: conn, rAddr: bindUDPAddr, tcpConn: control}
			return N.NewStreamPacketConn(pc, addr), nil
		})
		if err != nil {
			return nil, err
		}

		go func() {
			io.Copy(ioutil.Discard, control)
			control.Close()
			// A UDP association terminates when the TCP connection that the UDP
			// ASSOCIATE request arrived on terminates. RFC1928
			conn.Close()
		}()

		return WithRouteHop(conn, ss), err
	}

	return DialContextDecorated(ctx, network, ss.addr, func(conn net.Conn) (net.Conn, error) {
		if ss.tls {
			cc := tls.Client(conn, ss.tlsConfig)
			err := cc.Handshake()
			conn = cc
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
		if _, err := socks5.ClientHandshake(conn, socks5.ParseAddr(address), socks5.CmdConnect, user); err != nil {
			return nil, err
		}
		return WithRouteHop(conn, ss), nil
	})
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
	net.Conn
	rAddr   net.Addr
	tcpConn net.Conn
}

func (uc *socksPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	packet, err := socks5.EncodeUDPPacket(socks5.ParseAddrToSocksAddr(addr), b)
	if err != nil {
		return
	}
	return uc.Conn.Write(packet)
}

func (uc *socksPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, e := uc.Conn.Read(b)
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
	return uc.Conn.Close()
}
