package tunnel

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	C "github.com/Dreamacro/clash/constant"

	"github.com/Dreamacro/clash/common/pool"
)

func handleHTTP(request *adapters.HTTPAdapter, outbound net.Conn) {
	req := request.R
	host := req.Host

	inboundReader := bufio.NewReader(request)
	outboundReader := bufio.NewReader(outbound)

	for {
		keepAlive := strings.TrimSpace(strings.ToLower(req.Header.Get("Proxy-Connection"))) == "keep-alive"

		req.Header.Set("Connection", "close")
		req.RequestURI = ""
		adapters.RemoveHopByHopHeaders(req.Header)
		err := req.Write(outbound)
		if err != nil {
			break
		}

	handleResponse:
		resp, err := http.ReadResponse(outboundReader, req)
		if err != nil {
			break
		}
		adapters.RemoveHopByHopHeaders(resp.Header)

		if resp.StatusCode == http.StatusContinue {
			err = resp.Write(request)
			if err != nil {
				break
			}
			goto handleResponse
		}

		if keepAlive || resp.ContentLength >= 0 {
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

		// even if resp.Write write body to the connection, but some http request have to Copy to close it
		buf := pool.Get(pool.RelayBufferSize)
		_, err = io.CopyBuffer(request, resp.Body, buf)
		pool.Put(buf)
		if err != nil && err != io.EOF {
			break
		}

		req, err = http.ReadRequest(inboundReader)
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

func handleUDPToRemote(packet C.UDPPacket, pc C.PacketConn, metadata *C.Metadata) {
	defer packet.Drop()

	if _, err := pc.WriteWithMetadata(packet.Data(), metadata); err != nil {
		return
	}
}

func handleUDPToLocal(packet C.UDPPacket, pc net.PacketConn, key string, fAddr net.Addr) {
	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)
	defer natTable.Delete(key)
	defer pc.Close()

	for {
		pc.SetReadDeadline(time.Now().Add(udpTimeout))
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}

		if fAddr != nil {
			from = fAddr
		}

		n, err = packet.WriteBack(buf[:n], from)
		if err != nil {
			return
		}
	}
}

func handleSocket(request *adapters.SocketAdapter, outbound net.Conn) {
	relay(request, outbound)
}

// relay copies between left and right bidirectionally.
func relay(leftConn, rightConn net.Conn) {
	ch := make(chan error)

	go func() {
		buf := pool.Get(pool.RelayBufferSize)
		_, err := io.CopyBuffer(leftConn, rightConn, buf)
		pool.Put(buf)
		leftConn.SetReadDeadline(time.Now())
		ch <- err
	}()

	buf := pool.Get(pool.RelayBufferSize)
	io.CopyBuffer(rightConn, leftConn, buf)
	pool.Put(buf)
	rightConn.SetReadDeadline(time.Now())
	<-ch
}
