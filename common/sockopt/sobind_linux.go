package sockopt

import (
	"github.com/Dreamacro/clash/log"
	"net"
	"syscall"
)

func BindToDevice(dialer *net.Dialer, ifname string) {
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := syscall.BindToDevice(int(fd), ifname); err != nil {
				log.Errorln("Sockopt SO_BINDTODEVICE error: %s", err)
			}
		})
	}
}
