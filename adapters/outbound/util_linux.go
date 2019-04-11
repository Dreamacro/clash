package adapters

import (
	"net"
	"syscall"
	"time"

	"github.com/Dreamacro/clash/log"
	T "github.com/Dreamacro/clash/tunnel"
)

func dialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	socketMark := T.Instance().SocketMark()
	if socketMark == 0 {
		return net.DialTimeout(network, address, timeout)
	}
	d := &net.Dialer{
		Timeout: timeout,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, socketMark); err != nil {
					log.Errorln("Set SO_MARK error: %s", err)
				}
			})
		},
	}
	return d.Dial(network, address)
}
