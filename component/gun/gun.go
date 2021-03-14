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
	"time"

	"ekyu.moe/leb128"
	"github.com/Dreamacro/clash/log"
	"golang.org/x/net/http2"
)

type Conn struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
	local  net.Addr
	remote net.Addr
	done   chan struct{}
}

type Client struct {
	ctx     context.Context
	client  *http.Client
	url     *url.URL
	headers http.Header
}

type Config struct {
	ServiceName    string
	SkipCertVerify bool
	Tls            bool
	ServerName     string
	Adder          string
}

type ChainedClosable []io.Closer

// Close implements io.Closer.Close().
func (cc ChainedClosable) Close() error {
	for _, c := range cc {
		_ = c.Close()
	}
	return nil
}

var (
	ErrInvalidLength = errors.New("invalid length")
)

func (g *Conn) isClosed() bool {
	select {
	case <-g.done:
		return true
	default:
		return false
	}
}

func (g Conn) Read(b []byte) (n int, err error) {
	buf := make([]byte, 5)
	n, err = io.ReadFull(g.reader, buf)
	if err != nil {
		return 0, err
	}
	//log.Printf("GRPC Header: %x", buf[:n])
	grpcPayloadLen := binary.BigEndian.Uint32(buf[1:])
	//log.Printf("GRPC Payload Length: %d", grpcPayloadLen)

	buf = make([]byte, grpcPayloadLen)
	n, err = io.ReadFull(g.reader, buf)
	if err != nil {
		return 0, io.ErrUnexpectedEOF
	}
	protobufPayloadLen, protobufLengthLen := leb128.DecodeUleb128(buf[1:])
	//log.Printf("Protobuf Payload Length: %d, Length Len: %d", protobufPayloadLen, protobufLengthLen)
	if protobufLengthLen == 0 {
		return 0, ErrInvalidLength
	}
	if grpcPayloadLen != uint32(protobufPayloadLen)+uint32(protobufLengthLen)+1 {
		return 0, ErrInvalidLength
	}

	return bytes.NewReader(buf[1+protobufLengthLen:]).Read(b)
}

func (g Conn) Write(b []byte) (n int, err error) {
	if g.isClosed() {
		return 0, io.ErrClosedPipe
	}
	protobufHeader := leb128.AppendUleb128([]byte{0x0A}, uint64(len(b)))
	grpcHeader := make([]byte, 5)
	grpcPayloadLen := uint32(len(protobufHeader) + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)
	_, err = io.Copy(g.writer, io.MultiReader(bytes.NewReader(grpcHeader), bytes.NewReader(protobufHeader), bytes.NewReader(b)))
	if f, ok := g.writer.(http.Flusher); ok {
		f.Flush()
	}
	return len(b), err
}

func (g Conn) Close() error {
	defer close(g.done)
	err := g.closer.Close()
	return err
}

func (g Conn) LocalAddr() net.Addr {
	return g.local
}

func (g Conn) RemoteAddr() net.Addr {
	return g.remote
}

func (g Conn) SetDeadline(t time.Time) error {
	return nil
}

func (g Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (g Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

func StreamGunConn(cfg *Config) (net.Conn, error) {
	var dialFunc func(network, addr string, cfg *tls.Config) (net.Conn, error) = nil
	if cfg.Tls {
		dialFunc = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			pconn, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}

			cn := tls.Client(pconn, cfg)
			if err := cn.Handshake(); err != nil {
				return nil, err
			}
			state := cn.ConnectionState()
			if p := state.NegotiatedProtocol; p != http2.NextProtoTLS {
				return nil, errors.New("http2: unexpected ALPN protocol " + p + "; want q" + http2.NextProtoTLS)
			}
			return cn, nil
		}
	} else {
		dialFunc = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		}
	}

	var tlsClientConfig *tls.Config = nil
	if cfg.ServerName != "" {
		tlsClientConfig = new(tls.Config)
		tlsClientConfig.ServerName = cfg.ServerName
	}

	client := &http.Client{
		Transport: &http2.Transport{
			DialTLS:            dialFunc,
			TLSClientConfig:    tlsClientConfig,
			AllowHTTP:          false,
			DisableCompression: true,
			ReadIdleTimeout:    0,
			PingTimeout:        0,
		},
	}

	var serviceName = "GunService"
	if cfg.ServiceName != "" {
		serviceName = cfg.ServiceName
	}

	clientConn := &Client{
		ctx:    context.TODO(),
		client: client,
		url: &url.URL{
			Scheme: "https",
			Host:   cfg.Adder,
			Path:   fmt.Sprintf("/%s/Tun", serviceName),
		},
		headers: http.Header{
			"content-type": []string{"application/grpc"},
			"user-agent":   []string{"grpc-go/1.36.0"},
			"te":           []string{"trailers"},
		},
	}
	reader, writer := io.Pipe()
	request := &http.Request{
		Method:     http.MethodPost,
		Body:       reader,
		URL:        clientConn.url,
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     clientConn.headers,
	}
	anotherReader, anotherWriter := io.Pipe()
	go func() {
		defer anotherWriter.Close()
		response, err := clientConn.client.Do(request)
		if err != nil {
			log.Errorln("failed to dial remote: " + err.Error())
			return
		}
		_, _ = io.Copy(anotherWriter, response.Body)
	}()

	return &Conn{
		reader: anotherReader,
		writer: writer,
		closer: ChainedClosable{reader, writer, anotherReader},
		local: &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		},
		remote: &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		},
		done: make(chan struct{}),
	}, nil
}
