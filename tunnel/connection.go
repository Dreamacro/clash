package tunnel

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
)

func (t *Tunnel) handleHTTP(request *adapters.HTTPAdapter, outbound net.Conn) {
	conn := newTrafficTrack(outbound, t.traffic)
	req := request.R
	host := req.Host

	for {
		keepAlive := strings.TrimSpace(strings.ToLower(req.Header.Get("Proxy-Connection"))) == "keep-alive"

		req.Header.Set("Connection", "close")
		req.RequestURI = ""
		adapters.RemoveHopByHopHeaders(req.Header)
		err := req.Write(conn)
		if err != nil {
			break
		}
		br := bufio.NewReader(conn)
		resp, err := http.ReadResponse(br, req)
		if err != nil {
			break
		}
		adapters.RemoveHopByHopHeaders(resp.Header)
		if resp.ContentLength >= 0 {
			resp.Header.Set("Proxy-Connection", "keep-alive")
			resp.Header.Set("Connection", "keep-alive")
			resp.Header.Set("Keep-Alive", "timeout=4")
			resp.Close = false
		} else {
			resp.Close = true
		}
		err = resp.Write(request)
		if err != nil || resp.Close {
			break
		}

		if !keepAlive {
			break
		}

		req, err = http.ReadRequest(bufio.NewReader(request))
		if err != nil {
			break
		}

		// Sometimes firefox just open a socket to process multiple domains in HTTP
		// The temporary solution is close connection when encountering different HOST
		if req.Host != host {
			break
		}
	}
}

func (t *Tunnel) handleSocket(request *adapters.SocketAdapter, outbound net.Conn) {
	conn := newTrafficTrack(outbound, t.traffic)
	relay(request, conn)
}

// Reference: https://github.com/shadowsocks/go-shadowsocks2/tcp.go
// UDP: keep the connection until disconnect then free the UDP socket
func (t *Tunnel) handleUDPAssociate(conn net.Conn) {
	buf := make([]byte, 1)
	// block here
	for {
		_, err := conn.Read(buf)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			continue
		}
		return
	}
}

func (t *Tunnel) handleUDPToRemote(conn, pc net.Conn) {
	buf := pool.BufPool.Get().([]byte)
	defer pool.BufPool.Put(buf[:cap(buf)])
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	if _, err = pc.Write(buf[:n]); err != nil {
		return
	}
	t.traffic.Up() <- int64(n)
}

func (t *Tunnel) handleUDPToLocal(conn, pc net.Conn, target socks5.Addr) {
	src := newTrafficTrack(pc, t.Traffic())
	src.SetReadDeadline(time.Now().Add(udpNATMap.Timeout))

	buf := pool.BufPool.Get().([]byte)
	defer pool.BufPool.Put(buf[:cap(buf)])

	packet := pool.BufPool.Get().([]byte)
	defer pool.BufPool.Put(packet[:cap(packet)])

	for {
		n, err := src.Read(buf)
		if err != nil {
			break
		}

		n, err = socks5.EncodeUDPPacket(target.String(), buf[:n], packet)
		if err != nil {
			break
		}
		if _, err = conn.Write(packet[:n]); err != nil {
			break
		}
	}

	if pc := udpNATMap.Del(conn.RemoteAddr().String()); pc != nil {
		pc.Close()
	}
}

// relay copies between left and right bidirectionally.
func relay(leftConn, rightConn net.Conn) {
	ch := make(chan error)

	go func() {
		buf := pool.BufPool.Get().([]byte)
		_, err := io.CopyBuffer(leftConn, rightConn, buf)
		pool.BufPool.Put(buf[:cap(buf)])
		leftConn.SetReadDeadline(time.Now())
		ch <- err
	}()

	buf := pool.BufPool.Get().([]byte)
	io.CopyBuffer(rightConn, leftConn, buf)
	pool.BufPool.Put(buf[:cap(buf)])
	rightConn.SetReadDeadline(time.Now())
	<-ch
}
