package tunnel

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Dreamacro/clash/adapters/inbound"
	C "github.com/Dreamacro/clash/constant"
)

const (
	// io.Copy default buffer size is 32 KiB
	// but the maximum packet size of vmess/shadowsocks is about 16 KiB
	// so define a buffer of 20 KiB to reduce the memory of each TCP relay
	bufferSize = 20 * 1024
)

var bufPool = sync.Pool{New: func() interface{} { return make([]byte, bufferSize) }}

func (t *Tunnel) handleHTTP(request *adapters.HTTPAdapter, proxy C.ProxyAdapter) {
	conn := newTrafficTrack(proxy.Conn(), t.traffic)
	req := request.R
	host := req.Host
	keepalive := true

	for {
		if strings.ToLower(req.Header.Get("Connection")) == "close" {
			keepalive = false
		}

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
		err = resp.Write(request.Conn())
		if err != nil || resp.Close {
			break
		}

		if !keepalive {
			break
		}

		req, err = http.ReadRequest(bufio.NewReader(request.Conn()))
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

func (t *Tunnel) handleSOCKS(request *adapters.SocketAdapter, proxy C.ProxyAdapter) {
	conn := newTrafficTrack(proxy.Conn(), t.traffic)
	relay(request.Conn(), conn)
}

// relay copies between left and right bidirectionally.
func relay(leftConn, rightConn net.Conn) {
	ch := make(chan error)

	go func() {
		buf := bufPool.Get().([]byte)
		_, err := io.CopyBuffer(leftConn, rightConn, buf)
		bufPool.Put(buf[:cap(buf)])
		leftConn.SetReadDeadline(time.Now())
		ch <- err
	}()

	buf := bufPool.Get().([]byte)
	io.CopyBuffer(rightConn, leftConn, buf)
	bufPool.Put(buf[:cap(buf)])
	rightConn.SetReadDeadline(time.Now())
	<-ch
}
