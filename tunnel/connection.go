package tunnel

import (
	"errors"
	"io"
	"net"
	"time"

	N "github.com/Dreamacro/clash/common/net"
	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/resolver"
	C "github.com/Dreamacro/clash/constant"
)

func handleUDPToRemote(packet C.UDPPacket, pc C.PacketConn, metadata *C.Metadata) error {
	defer packet.Drop()

	// local resolve UDP dns
	if !metadata.Resolved() {
		ip, err := resolver.ResolveIP(metadata.Host)
		if err != nil {
			return err
		}
		metadata.DstIP = ip
	}

	addr := metadata.UDPAddr()
	if addr == nil {
		return errors.New("udp addr invalid")
	}

	if _, err := pc.WriteTo(packet.Data(), addr); err != nil {
		return err
	}
	// reset timeout
	pc.SetReadDeadline(time.Now().Add(udpTimeout))

	return nil
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

		_, err = packet.WriteBack(buf[:n], from)
		if err != nil {
			return
		}
	}
}

func handleSocket(ctx C.ConnContext, outbound net.Conn) {
	relay(ctx.Conn(), outbound)
}

// relay copies between left and right bidirectionally.
func relay(leftConn, rightConn net.Conn) {
	ch := make(chan error)

	go func() {
		buf := pool.Get(pool.RelayBufferSize)
		// Wrapping to avoid using *net.TCPConn.(ReadFrom)
		// See also https://github.com/Dreamacro/clash/pull/1209
		_, err := io.CopyBuffer(N.WriteOnlyWriter{Writer: leftConn}, N.ReadOnlyReader{Reader: rightConn}, buf)
		pool.Put(buf)
		leftConn.SetReadDeadline(time.Now())
		ch <- err
	}()

	buf := pool.Get(pool.RelayBufferSize)
	io.CopyBuffer(N.WriteOnlyWriter{Writer: rightConn}, N.ReadOnlyReader{Reader: leftConn}, buf)
	pool.Put(buf)
	rightConn.SetReadDeadline(time.Now())
	<-ch
}
