package redir

import (
	"fmt"
	"github.com/Dreamacro/clash/transport/socks5"
	"net"
	"syscall"
	"unsafe"
)

// https://gist.github.com/gkoyuncu/f8aad43f66815dac7769
// https://github.com/monsterxx03/pf_poc/blob/master/main.go

func parserPacket(c net.Conn) (socks5.Addr, error) {
	const (
		PfOut       = 2
		DIOCNATLOOK = 0xc04c4417
	)

	fd, err := syscall.Open("/dev/pf", 0, syscall.O_RDWR)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/df: %s", err)
	}
	defer syscall.Close(fd)

	/*
		struct pfioc_natlook {
			struct pf_addr   saddr;
			struct pf_addr   daddr;
			struct pf_addr   rsaddr;
			struct pf_addr   rdaddr;
			u_int16_t        sport;
			u_int16_t        dport;
			u_int16_t        rsport;
			u_int16_t        rdport;
			sa_family_t      af;
			u_int8_t         proto;
			u_int8_t         direction;
		};
	*/
	nl := struct { // struct pfioc_natlook
		saddr, daddr, rsaddr, rdaddr     [16]byte
		sxport, dxport, rsxport, rdxport [2]byte
		af, proto, direction             uint8
	}{
		af:        syscall.AF_INET,
		proto:     syscall.IPPROTO_TCP,
		direction: PfOut,
	}
	saddr := c.RemoteAddr().(*net.TCPAddr)
	daddr := c.LocalAddr().(*net.TCPAddr)
	copy(nl.saddr[:], saddr.IP)
	copy(nl.daddr[:], daddr.IP)
	nl.sxport[0], nl.sxport[1] = byte(saddr.Port>>8), byte(saddr.Port)
	nl.dxport[0], nl.dxport[1] = byte(daddr.Port>>8), byte(daddr.Port)

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), DIOCNATLOOK, uintptr(unsafe.Pointer(&nl))); errno != 0 {
		return nil, fmt.Errorf("ioctl failed: %s", errno)
	}

	addr := make([]byte, 1+net.IPv4len+2)
	addr[0] = socks5.AtypIPv4
	copy(addr[1:1+net.IPv4len], nl.rdaddr[:4])
	copy(addr[1+net.IPv4len:], nl.rdxport[:2])

	return addr, nil
}
