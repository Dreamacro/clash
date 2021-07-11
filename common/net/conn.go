package net

import "net"

type StreamPacketConn struct {
	net.PacketConn

	rAddr net.Addr
}

func (s *StreamPacketConn) Read(b []byte) (n int, err error) {
	for {
		n, addr, err := s.PacketConn.ReadFrom(b)
		if err != nil {
			return n, err
		}

		if addr.String() == s.rAddr.String() {
			return n, err
		}
	}
}

func (s *StreamPacketConn) Write(b []byte) (n int, err error) {
	return s.PacketConn.WriteTo(b, s.rAddr)
}

func (s *StreamPacketConn) RemoteAddr() net.Addr {
	return s.rAddr
}

func NewStreamPacketConn(pc net.PacketConn, remoteAddr net.Addr) *StreamPacketConn {
	return &StreamPacketConn{pc, remoteAddr}
}
