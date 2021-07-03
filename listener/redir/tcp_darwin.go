package redir

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/Dreamacro/clash/transport/socks5"
)

func parserPacket(c net.Conn) (socks5.Addr, error) {
	const (
		PfInout     = 0
		PfIn        = 1
		PfOut       = 2
		IOCOut      = 0x40000000
		IOCIn       = 0x80000000
		IOCInOut    = IOCIn | IOCOut
		IOCPARMMask = 0x1FFF
		LEN         = 4*16 + 4*4 + 4*1
		// #define	_IOC(inout,group,num,len) (inout | ((len & IOCPARMMask) << 16) | ((group) << 8) | (num))
		// #define	_IOWR(g,n,t)	_IOC(IOCInOut,	(g), (n), sizeof(t))
		// #define DIOCNATLOOK		_IOWR('D', 23, struct pfioc_natlook)
		DIOCNATLOOK = IOCInOut | ((LEN & IOCPARMMask) << 16) | ('D' << 8) | 23
	)

	fd, err := syscall.Open("/dev/pf", 0, syscall.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	nl := struct { // struct pfioc_natlook
		saddr, daddr, rsaddr, rdaddr       [16]byte
		sxport, dxport, rsxport, rdxport   [4]byte
		af, proto, protoVariant, direction uint8
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
		return nil, errno
	}

	addr := make([]byte, 1+net.IPv4len+2)
	addr[0] = socks5.AtypIPv4
	copy(addr[1:1+net.IPv4len], nl.rdaddr[:4])
	copy(addr[1+net.IPv4len:], nl.rdxport[:2])
	return addr, nil
}
