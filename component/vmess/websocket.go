package vmess

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type websocketConn struct {
	conn       *websocket.Conn
	reader     io.Reader
	remoteAddr net.Addr
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

func newWebsocketConn(conn *websocket.Conn, remoteAddr net.Addr) net.Conn {
	return &websocketConn{
		conn:       conn,
		remoteAddr: remoteAddr,
	}
}
