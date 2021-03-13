package proto

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"
)

type ClientConn struct {
	client GunService_TunClient
	reader io.Reader
	over   context.CancelFunc
}

func (s *ClientConn) LocalAddr() net.Addr {
	return nil
}

func (s *ClientConn) RemoteAddr() net.Addr {
	return nil
}

func (s *ClientConn) SetDeadline(t time.Time) error {
	return nil
}

func (s *ClientConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (s *ClientConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (s *ClientConn) Read(b []byte) (n int, err error) {
	if s.reader == nil {
		h, err := s.client.Recv()
		if err != nil {
			if err == io.EOF {
				s.reader = nil
				return n, err
			} else {
				return 0, errors.New("unable to read from gRPC tunnel")
			}
		}
		s.reader = bytes.NewReader(h.Data)
	}
	n, err = s.reader.Read(b)
	if err != nil {
		s.reader = nil
		return n, err
	}
	return n, err
}

func (s *ClientConn) Write(b []byte) (n int, err error) {
	err = s.client.Send(&Hunk{Data: b[:]})
	if err != nil {
		return 0, errors.New("unable to send data over gRPC")
	}
	return len(b), nil
}

func (s *ClientConn) Close() error {
	return s.client.CloseSend()
}

func NewClientConn(client GunService_TunClient) *ClientConn {
	return &ClientConn{
		client: client,
		reader: nil,
	}
}
