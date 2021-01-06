package dialer

import (
	"errors"
	"net"
)

var (
	errPlatformNotSupport = errors.New("unsupport platform")
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

func fallbackBindToDialer(dialer *net.Dialer, network string, ip net.IP, name string) error {
	if !ip.IsGlobalUnicast() {
		return nil
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return err
	}

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

func fallbackBindToListenConfig(name string) (string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, elm := range addrs {
		addr, ok := elm.(*net.IPNet)
		if !ok || addr.IP.To4() == nil {
			continue
		}

		return net.JoinHostPort(addr.IP.String(), "0"), nil
	}

	return "", ErrAddrNotFound
}
