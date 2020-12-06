// +build !linux

package dialer

import (
	"context"
	"net"
)

func Dialer() (*net.Dialer, error) {
	dialer := &net.Dialer{}
	if DialerHook != nil {
		if err := DialerHook(dialer); err != nil {
			return nil, err
		}
	}

	return dialer, nil
}

func ListenPacket(network, address string) (net.PacketConn, error) {
	cfg := &net.ListenConfig{}
	if ListenPacketHook != nil {
		var err error
		address, err = ListenPacketHook(cfg, address)
		if err != nil {
			return nil, err
		}
	}

	return cfg.ListenPacket(context.Background(), network, address)
}
