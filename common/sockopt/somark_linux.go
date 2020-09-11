package sockopt

import (
	"github.com/Dreamacro/clash/log"
	"net"
	"syscall"
)

func Mark(dialer *net.Dialer, mark int) {
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, mark); err != nil {
				log.Errorln("Sockopt SO_MARK error: %s", err)
			}
		})
	}
}
