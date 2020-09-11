package sockopt

import (
	"github.com/Dreamacro/clash/log"
	"net"
	"syscall"
)

func BindToDevice(dialer *net.Dialer, ifname string) {
	iface, err := net.InterfaceByName(ifname)

	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_BOUND_IF, iface.index); err != nil {
				log.Errorln("Sockopt IP_BOUND_IP error: %s", err)
			}
		})
	}
}
