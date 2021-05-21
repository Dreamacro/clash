package outbound

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/resolver"
	"github.com/Dreamacro/clash/component/vless"
	"github.com/Dreamacro/clash/component/vmess"
	C "github.com/Dreamacro/clash/constant"
)

const (
	// max packet length
	maxLength = 2046
)

var bufPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

type Vless struct {
	*Base
	client *vless.Client
	option *VlessOption
}

type VlessOption struct {
	Name           string            `proxy:"name"`
	Server         string            `proxy:"server"`
	Port           int               `proxy:"port"`
	UUID           string            `proxy:"uuid"`
	UDP            bool              `proxy:"udp,omitempty"`
	TLS            bool              `proxy:"tls,omitempty"`
	Network        string            `proxy:"network,omitempty"`
	WSPath         string            `proxy:"ws-path,omitempty"`
	WSHeaders      map[string]string `proxy:"ws-headers,omitempty"`
	SkipCertVerify bool              `proxy:"skip-cert-verify,omitempty"`
	ServerName     string            `proxy:"servername,omitempty"`
}

func (v *Vless) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
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
			wsOpts.SessionCache = getClientSessionCache()
			wsOpts.SkipCertVerify = v.option.SkipCertVerify
			wsOpts.ServerName = v.option.ServerName
		}
		c, err = vmess.StreamWebsocketConn(c, wsOpts)
	default:
		// handle TLS
		if v.option.TLS {
			host, _, _ := net.SplitHostPort(v.addr)
			tlsConfig := &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: v.option.SkipCertVerify,
				ClientSessionCache: getClientSessionCache(),
			}
			if v.option.ServerName != "" {
				tlsConfig.ServerName = v.option.ServerName
			}
			tlsConn := tls.Client(c, tlsConfig)
			if err = tlsConn.Handshake(); err != nil {
				return nil, err
			}

			c = tlsConn
		}
	}

	if err != nil {
		return nil, err
	}

	return v.client.StreamConn(c, parseVmessAddr(metadata))
}

func (v *Vless) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", v.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %s", v.addr, err.Error())
	}
	tcpKeepAlive(c)

	c, err = v.StreamConn(c, metadata)
	return NewConn(c, v), err
}

func (v *Vless) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	// vless use stream-oriented udp, so clash needs a net.UDPAddr
	if !metadata.Resolved() {
		ip, err := resolver.ResolveIP(metadata.Host)
		if err != nil {
			return nil, errors.New("can't resolve ip")
		}
		metadata.DstIP = ip
	}

	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	c, err := dialer.DialContext(ctx, "tcp", v.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %s", v.addr, err.Error())
	}
	tcpKeepAlive(c)
	c, err = v.StreamConn(c, metadata)
	if err != nil {
		return nil, fmt.Errorf("new vless client error: %v", err)
	}
	return newPacketConn(newVlessPacketConn(c, metadata.UDPAddr()), v), nil
}

func NewVless(option VlessOption) (*Vless, error) {
	client, err := vless.NewClient(option.UUID)
	if err != nil {
		return nil, err
	}

	return &Vless{
		Base: &Base{
			name: option.Name,
			addr: net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
			tp:   C.Vless,
			udp:  true,
		},
		client: client,
		option: &option,
	}, nil
}

func newVlessPacketConn(c net.Conn, addr net.Addr) *vlessPacketConn {
	return &vlessPacketConn{Conn: c,
		rAddr: addr,
	}
}

type vlessPacketConn struct {
	net.Conn
	rAddr  net.Addr
	remain int
	mux    sync.Mutex
}

func (c *vlessPacketConn) writePacket(b []byte, addr net.Addr) (int, error) {
	length := len(b)
	if length == 0 {
		return 0, nil
	}

	buffer := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buffer)
	defer buffer.Reset()

	buffer.WriteByte(byte(length >> 8))
	buffer.WriteByte(byte(length))
	buffer.Write(b)
	n, err := c.Conn.Write(buffer.Bytes())
	if n > 2 {
		return n - 2, err
	}

	return 0, err
}

func (c *vlessPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if len(b) <= maxLength {
		return c.writePacket(b, addr)
	}

	offset := 0
	total := len(b)
	for offset < total {
		cursor := offset + maxLength
		if cursor > total {
			cursor = total
		}

		n, err := c.writePacket(b[offset:cursor], addr)
		if err != nil {
			return offset + n, err
		}

		offset = cursor
	}

	return total, nil
}

func (c *vlessPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	length := len(b)
	if c.remain > 0 {
		if c.remain < length {
			length = c.remain
		}

		n, err := c.Conn.Read(b[:length])
		if err != nil {
			return 0, nil, err
		}

		c.remain -= n
		return n, c.rAddr, nil
	}

	var packetLength uint16
	if err := binary.Read(c.Conn, binary.BigEndian, &packetLength); err != nil {
		return 0, nil, err
	}

	remain := int(packetLength)
	n, err := c.Conn.Read(b[:length])
	remain -= n
	if remain > 0 {
		c.remain = remain
	}
	return n, c.rAddr, err
}
