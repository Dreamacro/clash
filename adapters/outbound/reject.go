package outbound

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

type Reject struct {
	*Base

	delay time.Duration
}

// DialContext implements C.ProxyAdapter
func (r *Reject) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	srcPort, _ := strconv.Atoi(metadata.SrcPort)
	dstPort, _ := strconv.Atoi(metadata.DstPort)

	mutex := &sync.Mutex{}

	nop := &NopConn{
		mutex:   mutex,
		cond:    sync.NewCond(mutex),
		timer:   time.NewTimer(r.delay),
		closed:  false,
		closeAt: time.Now().Add(r.delay),
		srcAddr: &net.TCPAddr{IP: metadata.SrcIP, Port: srcPort},
		dstAddr: &net.TCPAddr{IP: metadata.DstIP, Port: dstPort},
	}

	go nop.process()

	return NewConn(nop, r), nil
}

// DialUDP implements C.ProxyAdapter
func (r *Reject) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	return nil, errors.New("match reject rule")
}

func NewReject(delay time.Duration) *Reject {
	return &Reject{
		Base: &Base{
			name: "REJECT",
			tp:   C.Reject,
			udp:  true,
		},
		delay: delay,
	}
}

type NopConn struct {
	mutex *sync.Mutex
	cond  *sync.Cond
	timer *time.Timer

	closed  bool
	closeAt time.Time

	srcAddr net.Addr
	dstAddr net.Addr
}

func (rw *NopConn) Read(b []byte) (int, error) {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

	if !rw.closed {
		rw.cond.Wait()
	}

	if rw.closed {
		return 0, io.EOF
	}

	return 0, io.ErrNoProgress
}

func (rw *NopConn) Write(b []byte) (int, error) {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

	if rw.closed {
		return 0, io.EOF
	}

	return len(b), nil
}

// Close is fake function for net.Conn
func (rw *NopConn) Close() error {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

	if !rw.closed {
		rw.closed = true
		rw.cond.Broadcast()
	}

	return nil
}

// LocalAddr is fake function for net.Conn
func (rw *NopConn) LocalAddr() net.Addr { return rw.srcAddr }

// RemoteAddr is fake function for net.Conn
func (rw *NopConn) RemoteAddr() net.Addr { return rw.dstAddr }

// SetDeadline is fake function for net.Conn
func (rw *NopConn) SetDeadline(t time.Time) error {
	if t.After(rw.closeAt) {
		return nil
	}

	interval := time.Until(t)
	if interval < 0 {
		interval = 1
	}

	rw.timer.Reset(interval)

	return nil
}

// SetReadDeadline is fake function for net.Conn
func (rw *NopConn) SetReadDeadline(t time.Time) error { return rw.SetDeadline(t) }

// SetWriteDeadline is fake function for net.Conn
func (rw *NopConn) SetWriteDeadline(t time.Time) error { return rw.SetDeadline(t) }

func (rw *NopConn) process() {
	for {
		rw.mutex.Lock()
		closed := rw.closed
		rw.mutex.Unlock()

		if closed {
			return
		}

		interval := time.Until(rw.closeAt)
		if interval < 0 {
			interval = 1
		}

		rw.timer.Reset(interval)

		<-rw.timer.C

		rw.mutex.Lock()
		rw.cond.Broadcast()
		rw.mutex.Unlock()

		if rw.closeAt.Before(time.Now()) {
			rw.Close()
		}
	}
}
