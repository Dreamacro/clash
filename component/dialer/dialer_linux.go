package dialer

import (
	"context"
	"net"
	"syscall"
)

func Dialer() (*net.Dialer, error) {
	dialer := &net.Dialer{}
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, 0xff)
		})
	}
	if DialerHook != nil {
		if err := DialerHook(dialer); err != nil {
			return nil, err
		}
	}

	return dialer, nil
}

func ListenPacket(network, address string) (net.PacketConn, error) {
	cfg := &net.ListenConfig{}
	cfg.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, 0xff)
		})
	}
	if ListenPacketHook != nil {
		var err error
		address, err = ListenPacketHook(cfg, address)
		if err != nil {
			return nil, err
		}
	}

	return cfg.ListenPacket(context.Background(), network, address)
}
