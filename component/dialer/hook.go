package dialer

import (
	"errors"
	"net"
	"time"

	"github.com/Dreamacro/clash/common/singledo"
)

type DialerHookFunc = func(dialer *net.Dialer) error
type DialHookFunc = func(dialer *net.Dialer, network string, ip net.IP) error
type ListenConfigHookFunc = func(*net.ListenConfig) error
type ListenPacketHookFunc = func() (net.IP, error)

var (
	DialerHook       DialerHookFunc
	DialHook         DialHookFunc
	ListenConfigHook ListenConfigHookFunc
	ListenPacketHook ListenPacketHookFunc
)

var (
	ErrAddrNotFound      = errors.New("addr not found")
	ErrNetworkNotSupport = errors.New("network not support")
)

func lookupTCPAddr(ip net.IP, addrs []net.Addr) (*net.TCPAddr, error) {
	ipv4 := ip.To4() != nil

	for _, elm := range addrs {
		addr, ok := elm.(*net.IPNet)
		if !ok {
			continue
		}

		addrV4 := addr.IP.To4() != nil

		if addrV4 && ipv4 {
			return &net.TCPAddr{IP: addr.IP, Port: 0}, nil
		} else if !addrV4 && !ipv4 {
			return &net.TCPAddr{IP: addr.IP, Port: 0}, nil
		}
	}

	return nil, ErrAddrNotFound
}

func lookupUDPAddr(ip net.IP, addrs []net.Addr) (*net.UDPAddr, error) {
	ipv4 := ip.To4() != nil

	for _, elm := range addrs {
		addr, ok := elm.(*net.IPNet)
		if !ok {
			continue
		}

		addrV4 := addr.IP.To4() != nil

		if addrV4 && ipv4 {
			return &net.UDPAddr{IP: addr.IP, Port: 0}, nil
		} else if !addrV4 && !ipv4 {
			return &net.UDPAddr{IP: addr.IP, Port: 0}, nil
		}
	}

	return nil, ErrAddrNotFound
}

func ListenPacketWithInterface(name string) ListenPacketHookFunc {
	single := singledo.NewSingle(5 * time.Second)

	return func() (net.IP, error) {
		elm, err, _ := single.Do(func() (interface{}, error) {
			iface, err := net.InterfaceByName(name)
			if err != nil {
				return nil, err
			}

			addrs, err := iface.Addrs()
			if err != nil {
				return nil, err
			}

			return addrs, nil
		})

		if err != nil {
			return nil, err
		}

		addrs := elm.([]net.Addr)

		for _, elm := range addrs {
			addr, ok := elm.(*net.IPNet)
			if !ok || addr.IP.To4() == nil {
				continue
			}

			return addr.IP, nil
		}

		return nil, ErrAddrNotFound
	}
}

func DialerWithInterface(name string) DialHookFunc {
	single := singledo.NewSingle(5 * time.Second)

	return func(dialer *net.Dialer, network string, ip net.IP) error {
		elm, err, _ := single.Do(func() (interface{}, error) {
			iface, err := net.InterfaceByName(name)
			if err != nil {
				return nil, err
			}

			addrs, err := iface.Addrs()
			if err != nil {
				return nil, err
			}

			return addrs, nil
		})

		if err != nil {
			return err
		}

		addrs := elm.([]net.Addr)

		switch network {
		case "tcp", "tcp4", "tcp6":
			if addr, err := lookupTCPAddr(ip, addrs); err == nil {
				dialer.LocalAddr = addr
			} else {
				return err
			}
		case "udp", "udp4", "udp6":
			if addr, err := lookupUDPAddr(ip, addrs); err == nil {
				dialer.LocalAddr = addr
			} else {
				return err
			}
		}

		return nil
	}
}
