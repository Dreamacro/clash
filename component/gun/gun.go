// Modified from: https://github.com/Qv2ray/gun-lite
// License: MIT

package gun

import (
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

	"github.com/Dreamacro/clash/common/pool"

	"go.uber.org/atomic"
	"golang.org/x/net/http2"
)

var (
	ErrInvalidLength = errors.New("invalid length")
)

var (
	defaultHeader = http.Header{
		"content-type": []string{"application/grpc"},
		"user-agent":   []string{"grpc-go/1.36.0"},
	}
)

type DialFn = func(network, addr string) (net.Conn, error)

type Conn struct {
	response *http.Response
	request  *http.Request
	client   *http.Client
	writer   *io.PipeWriter
	once     sync.Once
	close    *atomic.Bool
	err      error

	buf    []byte
	offset int
}

type Config struct {
	ServiceName string
	Host        string
}

func (g *Conn) initRequest() {
	response, err := g.client.Do(g.request)
	if err != nil {
		g.err = err
		g.writer.Close()
		return
	}

	if !g.close.Load() {
		g.response = response
	} else {
		response.Body.Close()
	}
}

func (g *Conn) Read(b []byte) (n int, err error) {
	g.once.Do(g.initRequest)
	if g.err != nil {
		return 0, g.err
	}

	if g.buf != nil {
		n = copy(b, g.buf[g.offset:])
		g.offset += n
		if g.offset == len(g.buf) {
			g.offset = 0
			g.buf = nil
		}
		return
	}

	buf := make([]byte, 5)
	_, err = io.ReadFull(g.response.Body, buf)
	if err != nil {
		return 0, err
	}
	grpcPayloadLen := binary.BigEndian.Uint32(buf[1:])
	if grpcPayloadLen > pool.RelayBufferSize {
		return 0, ErrInvalidLength
	}

	buf = pool.Get(int(grpcPayloadLen))
	_, err = io.ReadFull(g.response.Body, buf)
	if err != nil {
		pool.Put(buf)
		return 0, io.ErrUnexpectedEOF
	}
	protobufPayloadLen, protobufLengthLen := decodeUleb128(buf[1:])
	if protobufLengthLen == 0 {
		pool.Put(buf)
		return 0, ErrInvalidLength
	}
	if grpcPayloadLen != uint32(protobufPayloadLen)+uint32(protobufLengthLen)+1 {
		pool.Put(buf)
		return 0, ErrInvalidLength
	}

	if len(b) >= int(grpcPayloadLen)-1-int(protobufLengthLen) {
		n = copy(b, buf[1+protobufLengthLen:])
		pool.Put(buf)
		return
	}
	n = copy(b, buf[1+protobufLengthLen:])
	g.offset = n + 1 + int(protobufLengthLen)
	g.buf = buf
	return
}

func (g *Conn) Write(b []byte) (n int, err error) {
	protobufHeader := appendUleb128([]byte{0x0A}, uint64(len(b)))
	grpcHeader := make([]byte, 5)
	grpcPayloadLen := uint32(len(protobufHeader) + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)

	buffers := net.Buffers{grpcHeader, protobufHeader, b}
	_, err = buffers.WriteTo(g.writer)
	if err == io.ErrClosedPipe && g.err != nil {
		err = g.err
	}

	return len(b), err
}

func (g *Conn) Close() error {
	g.close.Store(true)
	if r := g.response; r != nil {
		r.Body.Close()
	}

	return g.writer.Close()
}

func (g *Conn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.IPv4zero, Port: 0} }
func (g *Conn) RemoteAddr() net.Addr               { return &net.TCPAddr{IP: net.IPv4zero, Port: 0} }
func (g *Conn) SetDeadline(t time.Time) error      { return nil }
func (g *Conn) SetReadDeadline(t time.Time) error  { return nil }
func (g *Conn) SetWriteDeadline(t time.Time) error { return nil }

func NewHTTP2Client(dialFn DialFn, tlsConfig *tls.Config) *http2.Transport {
	dialFunc := func(network, addr string, cfg *tls.Config) (net.Conn, error) {
		pconn, err := dialFn(network, addr)
		if err != nil {
			return nil, err
		}

		cn := tls.Client(pconn, cfg)
		if err := cn.Handshake(); err != nil {
			pconn.Close()
			return nil, err
		}
		state := cn.ConnectionState()
		if p := state.NegotiatedProtocol; p != http2.NextProtoTLS {
			cn.Close()
			return nil, errors.New("http2: unexpected ALPN protocol " + p + "; want q" + http2.NextProtoTLS)
		}
		return cn, nil
	}

	return &http2.Transport{
		DialTLS:            dialFunc,
		TLSClientConfig:    tlsConfig,
		AllowHTTP:          false,
		DisableCompression: true,
		ReadIdleTimeout:    0,
		PingTimeout:        0,
	}
}

func StreamGunWithTransport(transport *http2.Transport, cfg *Config) (net.Conn, error) {
	serviceName := "GunService"
	if cfg.ServiceName != "" {
		serviceName = cfg.ServiceName
	}

	client := &http.Client{
		Transport: transport,
	}

	reader, writer := io.Pipe()
	request := &http.Request{
		Method: http.MethodPost,
		Body:   reader,
		URL: &url.URL{
			Scheme: "https",
			Host:   cfg.Host,
			Path:   fmt.Sprintf("/%s/Tun", serviceName),
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     defaultHeader,
	}

	conn := &Conn{
		request: request,
		client:  client,
		writer:  writer,
		close:   atomic.NewBool(false),
	}

	go conn.once.Do(conn.initRequest)
	return conn, nil
}

func StreamGunWithConn(conn net.Conn, tlsConfig *tls.Config, cfg *Config) (net.Conn, error) {
	dialFn := func(network, addr string) (net.Conn, error) {
		return conn, nil
	}

	transport := NewHTTP2Client(dialFn, tlsConfig)
	return StreamGunWithTransport(transport, cfg)
}
