// Modified from: https://github.com/Qv2ray/gun-lite
// License: MIT

package gun

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	N "github.com/Dreamacro/clash/common/net"
	"golang.org/x/net/http2"
)

var (
	ErrInvalidLength = errors.New("invalid length")
	ErrClientOnly    = errors.New("stream client only")
)

var (
	defaultHeader = http.Header{
		"content-type": []string{"application/grpc"},
		"user-agent":   []string{"grpc-go/1.36.0"},
	}
	bufferPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
)

type Config struct {
	ServiceName string
	Host        string
}

type Gun struct {
	*Config
	transport *http2.Transport
}

type Trunk struct {
	conn   net.Conn
	client *http2.ClientConn
	gun    *Gun
}

type conn struct {
	lAddr     net.Addr
	rAddr     net.Addr
	readable  sync.Mutex
	reader    *N.BufferReadCloser
	writer    io.WriteCloser
	remain    int
	deadLock  sync.Mutex
	deadTimer *time.Timer
}

func (g *Gun) NewTrunk(conn net.Conn) (*Trunk, error) {
	cc := tls.Client(conn, g.transport.TLSClientConfig)
	if err := cc.Handshake(); err != nil {
		return nil, err
	}

	if state := cc.ConnectionState(); state.NegotiatedProtocol != http2.NextProtoTLS {
		return nil, fmt.Errorf("http2: unexpected ALPN protocol %s, want %s", state.NegotiatedProtocol, http2.NextProtoTLS)
	}

	conn = cc

	client, err := g.transport.NewClientConn(conn)
	if err != nil {
		return nil, err
	}

	return &Trunk{
		conn:   conn,
		client: client,
		gun:    g,
	}, nil
}

func (t *Trunk) NewConn(ctx context.Context) (net.Conn, error) {
	serviceName := "GunService"
	if t.gun.ServiceName != "" {
		serviceName = t.gun.ServiceName
	}

	reader, writer := io.Pipe()
	request := &http.Request{
		Method: http.MethodPost,
		Body:   reader,
		URL: &url.URL{
			Scheme: "https",
			Host:   t.gun.Host,
			Path:   fmt.Sprintf("/%s/Tun", serviceName),
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     defaultHeader,
	}
	request = request.WithContext(ctx)

	c := &conn{
		lAddr:  t.conn.LocalAddr(),
		rAddr:  t.conn.RemoteAddr(),
		writer: writer,
	}

	c.readable.Lock()
	go func() {
		defer c.readable.Unlock()

		resp, err := t.client.RoundTrip(request)
		if err != nil {
			c.reader = N.NewBufferReadCloser(io.NopCloser(io.LimitReader(nil, 0)))

			return
		}

		c.reader = N.NewBufferReadCloser(resp.Body)
	}()

	return c, nil
}

func (t *Trunk) Close() error {
	t.client.Close()
	return t.conn.Close()
}

func (c *conn) Read(b []byte) (n int, err error) {
	c.readable.Lock()
	defer c.readable.Unlock()

	if c.remain > 0 {
		size := c.remain
		if len(b) < size {
			size = len(b)
		}

		n, err = io.ReadFull(c.reader, b[:size])
		c.remain -= n
		return
	}

	// 0x00 grpclength(uint32) 0x0A uleb128 payload
	_, err = c.reader.Discard(6)
	if err != nil {
		return 0, err
	}

	protobufPayloadLen, err := binary.ReadUvarint(c.reader)
	if err != nil {
		return 0, ErrInvalidLength
	}

	size := int(protobufPayloadLen)
	if len(b) < size {
		size = len(b)
	}

	n, err = io.ReadFull(c.reader, b[:size])
	if err != nil {
		return
	}

	remain := int(protobufPayloadLen) - n
	if remain > 0 {
		c.remain = remain
	}

	return n, nil
}

func (c *conn) Write(b []byte) (int, error) {
	protobufHeader := [binary.MaxVarintLen64 + 1]byte{0x0A}
	varuintSize := binary.PutUvarint(protobufHeader[1:], uint64(len(b)))
	grpcHeader := make([]byte, 5)
	grpcPayloadLen := uint32(varuintSize + 1 + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)

	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf)
	defer buf.Reset()
	buf.Write(grpcHeader)
	buf.Write(protobufHeader[:varuintSize+1])
	buf.Write(b)

	return c.writer.Write(buf.Bytes())
}

func (c *conn) Close() error {
	c.writer.Close()

	c.readable.Lock()
	defer c.readable.Unlock()

	c.reader.Close()

	return nil
}

func (c *conn) LocalAddr() net.Addr {
	return c.lAddr
}

func (c *conn) RemoteAddr() net.Addr {
	return c.rAddr
}

func (c *conn) SetDeadline(t time.Time) error {
	c.deadLock.Lock()
	defer c.deadLock.Unlock()

	if c.deadTimer != nil {
		c.deadTimer.Stop()
		c.deadTimer = nil
	}

	if !t.IsZero() {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}

		c.deadTimer = time.AfterFunc(d, func() {
			c.Close()
		})
	}

	return nil
}

func (c *conn) SetReadDeadline(t time.Time) error {
	return c.SetDeadline(t)
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	return c.SetDeadline(t)
}

func New(tlsCfg *tls.Config, cfg *Config) *Gun {
	hasHttp2 := false
	for _, proto := range tlsCfg.NextProtos {
		if proto == http2.NextProtoTLS {
			hasHttp2 = true
			break
		}
	}
	if !hasHttp2 {
		tlsCfg.NextProtos = append([]string{http2.NextProtoTLS}, tlsCfg.NextProtos...)
	}
	if tlsCfg.ServerName == "" {
		tlsCfg.ServerName = cfg.ServiceName
	}
	return &Gun{
		Config: cfg,
		transport: &http2.Transport{
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return nil, ErrClientOnly
			},
			TLSClientConfig:    tlsCfg,
			AllowHTTP:          false,
			DisableCompression: true,
			PingTimeout:        0,
		},
	}
}
