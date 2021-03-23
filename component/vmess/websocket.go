package vmess

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type websocketConn struct {
	conn       *websocket.Conn
	reader     io.Reader
	remoteAddr net.Addr

	// https://godoc.org/github.com/gorilla/websocket#hdr-Concurrency
	rMux sync.Mutex
	wMux sync.Mutex
}

// websocketEDConn websocket 0-rtt
type websocketEDConn struct {
	net.Conn
	realConn net.Conn
	closed   bool
	dialed   chan bool
	cancel   context.CancelFunc
	ctx      context.Context
	config   *WebsocketConfig
}

type WebsocketConfig struct {
	Host           string
	Port           string
	Path           string
	Headers        http.Header
	TLS            bool
	SkipCertVerify bool
	ServerName     string
	SessionCache   tls.ClientSessionCache
	Ed             uint32
}

// Read implements net.Conn.Read()
func (wsc *websocketConn) Read(b []byte) (int, error) {
	wsc.rMux.Lock()
	defer wsc.rMux.Unlock()
	for {
		reader, err := wsc.getReader()
		if err != nil {
			return 0, err
		}

		nBytes, err := reader.Read(b)
		if err == io.EOF {
			wsc.reader = nil
			continue
		}
		return nBytes, err
	}
}

// Write implements io.Writer.
func (wsc *websocketConn) Write(b []byte) (int, error) {
	wsc.wMux.Lock()
	defer wsc.wMux.Unlock()
	if err := wsc.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (wsc *websocketConn) Close() error {
	var errors []string
	if err := wsc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second*5)); err != nil {
		errors = append(errors, err.Error())
	}
	if err := wsc.conn.Close(); err != nil {
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to close connection: %s", strings.Join(errors, ","))
	}
	return nil
}

func (wsc *websocketConn) getReader() (io.Reader, error) {
	if wsc.reader != nil {
		return wsc.reader, nil
	}

	_, reader, err := wsc.conn.NextReader()
	if err != nil {
		return nil, err
	}
	wsc.reader = reader
	return reader, nil
}

func (wsc *websocketConn) LocalAddr() net.Addr {
	return wsc.conn.LocalAddr()
}

func (wsc *websocketConn) RemoteAddr() net.Addr {
	return wsc.remoteAddr
}

func (wsc *websocketConn) SetDeadline(t time.Time) error {
	if err := wsc.SetReadDeadline(t); err != nil {
		return err
	}
	return wsc.SetWriteDeadline(t)
}

func (wsc *websocketConn) SetReadDeadline(t time.Time) error {
	return wsc.conn.SetReadDeadline(t)
}

func (wsc *websocketConn) SetWriteDeadline(t time.Time) error {
	return wsc.conn.SetWriteDeadline(t)
}

func StreamWebsocketEDConn(conn net.Conn, c *WebsocketConfig) (net.Conn, error) {
	ctx, cancel := context.WithCancel(context.Background())
	conn = &websocketEDConn{
		dialed:   make(chan bool, 1),
		cancel:   cancel,
		ctx:      ctx,
		realConn: conn,
		config:   c,
	}
	return conn, nil
}

func (wsedc *websocketEDConn) Close() error {
	wsedc.closed = true
	wsedc.cancel()
	if wsedc.Conn == nil {
		return nil
	}
	return wsedc.Conn.Close()
}

func (wsedc *websocketEDConn) Write(b []byte) (int, error) {
	if wsedc.closed {
		return 0, io.ErrClosedPipe
	}
	if wsedc.Conn == nil {
		ed := b
		if len(ed) > int(wsedc.config.Ed) {
			ed = nil
		}
		var err error
		if wsedc.Conn, err = StreamWebsocketConn(wsedc.realConn, wsedc.config, ed); err != nil {
			wsedc.Close()
			return 0, errors.New("failed to dial WebSocket: " + err.Error())
		}
		wsedc.dialed <- true
		if ed != nil {
			return len(ed), nil
		}
	}
	return wsedc.Conn.Write(b)
}

func (wsedc *websocketEDConn) Read(b []byte) (int, error) {
	if wsedc.closed {
		return 0, io.ErrClosedPipe
	}
	if wsedc.Conn == nil {
		select {
		case <-wsedc.ctx.Done():
			return 0, io.ErrUnexpectedEOF
		case <-wsedc.dialed:
		}
	}
	return wsedc.Conn.Read(b)
}

func (wsedc *websocketEDConn) LocalAddr() net.Addr {
	if wsedc.Conn == nil {
		return wsedc.realConn.LocalAddr()
	}
	return wsedc.Conn.LocalAddr()
}

func (wsedc *websocketEDConn) RemoteAddr() net.Addr {
	if wsedc.Conn == nil {
		return wsedc.realConn.RemoteAddr()
	}
	return wsedc.Conn.RemoteAddr()
}

func (wsedc *websocketEDConn) SetDeadline(t time.Time) error {
	if err := wsedc.SetReadDeadline(t); err != nil {
		return err
	}
	return wsedc.SetWriteDeadline(t)
}

func (wsedc *websocketEDConn) SetReadDeadline(t time.Time) error {
	if wsedc.Conn == nil {
		return nil
	}
	return wsedc.Conn.SetReadDeadline(t)
}

func (wsedc *websocketEDConn) SetWriteDeadline(t time.Time) error {
	if wsedc.Conn == nil {
		return nil
	}
	return wsedc.Conn.SetWriteDeadline(t)
}

func StreamWebsocketConn(conn net.Conn, c *WebsocketConfig, ed []byte) (net.Conn, error) {
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return conn, nil
		},
		ReadBufferSize:   4 * 1024,
		WriteBufferSize:  4 * 1024,
		HandshakeTimeout: time.Second * 8,
	}

	scheme := "ws"
	if c.TLS {
		scheme = "wss"
		dialer.TLSClientConfig = &tls.Config{
			ServerName:         c.Host,
			InsecureSkipVerify: c.SkipCertVerify,
			ClientSessionCache: c.SessionCache,
		}

		if c.ServerName != "" {
			dialer.TLSClientConfig.ServerName = c.ServerName
		} else if host := c.Headers.Get("Host"); host != "" {
			dialer.TLSClientConfig.ServerName = host
		}
	}

	uri := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   c.Path,
	}

	headers := http.Header{}
	if c.Headers != nil {
		for k := range c.Headers {
			headers.Add(k, c.Headers.Get(k))
		}
	}

	if ed != nil {
		headers.Set("Sec-WebSocket-Protocol", base64.StdEncoding.EncodeToString(ed))
	}

	wsConn, resp, err := dialer.Dial(uri.String(), headers)
	if err != nil {
		reason := err.Error()
		if resp != nil {
			reason = resp.Status
		}
		return nil, fmt.Errorf("dial %s error: %s", uri.Host, reason)
	}

	return &websocketConn{
		conn:       wsConn,
		remoteAddr: conn.RemoteAddr(),
	}, nil
}
