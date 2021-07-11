package outbound

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	N "github.com/Dreamacro/clash/common/net"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/gun"
	"github.com/Dreamacro/clash/transport/socks5"
	"github.com/Dreamacro/clash/transport/vmess"
)

type Vmess struct {
	*Base
	client *vmess.Client
	option *VmessOption
	gun    *gun.Pool // for gun mux
}

type VmessOption struct {
	Name           string            `proxy:"name"`
	Server         string            `proxy:"server"`
	Port           int               `proxy:"port"`
	UUID           string            `proxy:"uuid"`
	AlterID        int               `proxy:"alterId"`
	Cipher         string            `proxy:"cipher"`
	TLS            bool              `proxy:"tls,omitempty"`
	UDP            bool              `proxy:"udp,omitempty"`
	Network        string            `proxy:"network,omitempty"`
	HTTPOpts       HTTPOptions       `proxy:"http-opts,omitempty"`
	HTTP2Opts      HTTP2Options      `proxy:"h2-opts,omitempty"`
	GrpcOpts       GrpcOptions       `proxy:"grpc-opts,omitempty"`
	WSPath         string            `proxy:"ws-path,omitempty"`
	WSHeaders      map[string]string `proxy:"ws-headers,omitempty"`
	SkipCertVerify bool              `proxy:"skip-cert-verify,omitempty"`
	ServerName     string            `proxy:"servername,omitempty"`
}

type HTTPOptions struct {
	Method  string              `proxy:"method,omitempty"`
	Path    []string            `proxy:"path,omitempty"`
	Headers map[string][]string `proxy:"headers,omitempty"`
}

type HTTP2Options struct {
	Host []string `proxy:"host,omitempty"`
	Path string   `proxy:"path,omitempty"`
}

type GrpcOptions struct {
	GrpcServiceName string `proxy:"grpc-service-name,omitempty"`
}

// DialContext implements C.ProxyAdapter
func (v *Vmess) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	addr, err := resolveVmessAddr(network, address)
	if err != nil {
		return nil, err
	}

	var conn net.Conn

	// gun transport
	if v.gun != nil {
		trunk, err := v.gun.GetTrunk(ctx)
		if err != nil {
			return nil, err
		}

		conn, err = trunk.NewConn(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %s", v.addr, err.Error())
		}

		c, err := v.client.StreamConn(conn, addr)
		if err != nil {
			conn.Close()

			return nil, err
		}

		conn = c
	} else {
		conn, err = DialContextDecorated(ctx, "tcp", v.addr, func(conn net.Conn) (net.Conn, error) {
			var err error

			switch v.option.Network {
			case "ws":
				host, port, _ := net.SplitHostPort(v.addr)
				wsOpts := &vmess.WebsocketConfig{
					Host: host,
					Port: port,
					Path: v.option.WSPath,
				}

				if len(v.option.WSHeaders) != 0 {
					header := http.Header{}
					for key, value := range v.option.WSHeaders {
						header.Add(key, value)
					}
					wsOpts.Headers = header
				}

				if v.option.TLS {
					wsOpts.TLS = true
					wsOpts.SkipCertVerify = v.option.SkipCertVerify
					wsOpts.ServerName = v.option.ServerName
				}
				conn, err = vmess.StreamWebsocketConn(conn, wsOpts)
			case "http":
				// readability first, so just copy default TLS logic
				if v.option.TLS {
					host, _, _ := net.SplitHostPort(v.addr)
					tlsOpts := &vmess.TLSConfig{
						Host:           host,
						SkipCertVerify: v.option.SkipCertVerify,
					}

					if v.option.ServerName != "" {
						tlsOpts.Host = v.option.ServerName
					}

					conn, err = vmess.StreamTLSConn(conn, tlsOpts)
					if err != nil {
						break
					}
				}

				host, _, _ := net.SplitHostPort(v.addr)
				httpOpts := &vmess.HTTPConfig{
					Host:    host,
					Method:  v.option.HTTPOpts.Method,
					Path:    v.option.HTTPOpts.Path,
					Headers: v.option.HTTPOpts.Headers,
				}

				conn = vmess.StreamHTTPConn(conn, httpOpts)
			case "h2":
				host, _, _ := net.SplitHostPort(v.addr)
				tlsOpts := vmess.TLSConfig{
					Host:           host,
					SkipCertVerify: v.option.SkipCertVerify,
					NextProtos:     []string{"h2"},
				}

				if v.option.ServerName != "" {
					tlsOpts.Host = v.option.ServerName
				}

				conn, err = vmess.StreamTLSConn(conn, &tlsOpts)
				if err != nil {
					break
				}

				h2Opts := &vmess.H2Config{
					Hosts: v.option.HTTP2Opts.Host,
					Path:  v.option.HTTP2Opts.Path,
				}

				conn, err = vmess.StreamH2Conn(conn, h2Opts)
			default:
				// handle TLS
				if v.option.TLS {
					host, _, _ := net.SplitHostPort(v.addr)
					tlsOpts := &vmess.TLSConfig{
						Host:           host,
						SkipCertVerify: v.option.SkipCertVerify,
					}

					if v.option.ServerName != "" {
						tlsOpts.Host = v.option.ServerName
					}

					conn, err = vmess.StreamTLSConn(conn, tlsOpts)
				}
			}
			if err != nil {
				return nil, err
			}

			return v.client.StreamConn(conn, addr)
		})
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %s", v.addr, err.Error())
		}
	}

	if addr.UDP {
		rAddr := &net.UDPAddr{
			IP:   addr.Addr,
			Port: int(addr.Port),
		}

		conn = N.NewStreamPacketConn(&vmessPacketConn{conn, rAddr}, rAddr)
	}

	return WithRouteHop(conn, v), nil
}

func NewVmess(option VmessOption) (*Vmess, error) {
	security := strings.ToLower(option.Cipher)
	client, err := vmess.NewClient(vmess.Config{
		UUID:     option.UUID,
		AlterID:  uint16(option.AlterID),
		Security: security,
		HostName: option.Server,
		Port:     strconv.Itoa(option.Port),
		IsAead:   option.AlterID == 0,
	})
	if err != nil {
		return nil, err
	}

	switch option.Network {
	case "h2", "grpc":
		if !option.TLS {
			return nil, fmt.Errorf("TLS must be true with h2/grpc network")
		}
	}

	v := &Vmess{
		Base: &Base{
			name: option.Name,
			addr: net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
			tp:   C.Vmess,
			udp:  option.UDP,
		},
		client: client,
		option: &option,
	}

	switch option.Network {
	case "h2":
		if len(option.HTTP2Opts.Host) == 0 {
			option.HTTP2Opts.Host = append(option.HTTP2Opts.Host, "www.example.com")
		}
	case "grpc":
		gunConfig := &gun.Config{
			ServiceName: v.option.GrpcOpts.GrpcServiceName,
			Host:        v.option.ServerName,
		}
		tlsConfig := &tls.Config{
			InsecureSkipVerify: v.option.SkipCertVerify,
			ServerName:         v.option.ServerName,
		}

		if v.option.ServerName == "" {
			host, _, _ := net.SplitHostPort(v.addr)
			tlsConfig.ServerName = host
			gunConfig.Host = host
		}

		g := gun.New(tlsConfig, gunConfig)

		v.gun = gun.NewPool(func(ctx context.Context) (*gun.Trunk, error) {
			conn, err := DialContext(ctx, "tcp", v.addr)
			if err != nil {
				return nil, fmt.Errorf("%s connect error: %s", v.addr, err.Error())
			}

			t, err := g.NewTrunk(conn)
			if err != nil {
				conn.Close()

				return nil, err
			}

			return t, nil
		})
	}

	return v, nil
}

func resolveVmessAddr(network, address string) (*vmess.DstAddr, error) {
	udp := network == "udp" || network == "udp4" || network == "udp6"
	if udp {
		addr, err := resolveUDPAddr("udp", address)
		if err != nil {
			return nil, err
		}
		address = addr.String()
	}

	socks5Addr := socks5.ParseAddr(address)
	if socks5Addr == nil {
		return nil, errors.New("invalid socks5 address")
	}

	var addrType byte
	switch socks5Addr[0] {
	case C.AtypIPv4:
		addrType = vmess.AtypIPv4
	case C.AtypIPv6:
		addrType = vmess.AtypIPv6
	case C.AtypDomainName:
		addrType = vmess.AtypDomainName
	}

	return &vmess.DstAddr{
		UDP:      udp,
		AddrType: addrType,
		Addr:     socks5Addr[1 : len(socks5Addr)-2],
		Port:     uint(binary.BigEndian.Uint16(socks5Addr[len(socks5Addr)-2:])),
	}, nil
}

type vmessPacketConn struct {
	net.Conn
	rAddr net.Addr
}

func (uc *vmessPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return uc.Conn.Write(b)
}

func (uc *vmessPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, err := uc.Conn.Read(b)
	return n, uc.rAddr, err
}
