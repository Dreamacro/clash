package vmess

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type websocketConn struct {
	conn       *websocket.Conn
	reader     io.Reader
	remoteAddr net.Addr
}

type websocketConfig struct {
	host      string
	path      string
	tls       bool
	tlsConfig *tls.Config
}

// Read implements net.Conn.Read()
func (wsc *websocketConn) Read(b []byte) (int, error) {
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
		return fmt.Errorf("Failed to close connection: %s", strings.Join(errors, ","))
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

func newWebsocketConn(conn net.Conn, c *websocketConfig) (net.Conn, error) {
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return conn, nil
		},
		ReadBufferSize:   4 * 1024,
		WriteBufferSize:  4 * 1024,
		HandshakeTimeout: time.Second * 8,
	}

	scheme := "ws"
	if c.tls {
		scheme = "wss"
		dialer.TLSClientConfig = c.tlsConfig
	}

	host, port, err := net.SplitHostPort(c.host)
	if (scheme == "ws" && port != "80") || (scheme == "wss" && port != "443") {
		host = c.host
	}

	uri := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   c.path,
	}

	wsConn, resp, err := dialer.Dial(uri.String(), nil)
	if err != nil {
		var reason string
		if resp != nil {
			reason = resp.Status
		}
		return nil, fmt.Errorf("Dial %s error: %s", host, reason)
	}

	return &websocketConn{
		conn:       wsConn,
		remoteAddr: conn.RemoteAddr(),
	}, nil
}
