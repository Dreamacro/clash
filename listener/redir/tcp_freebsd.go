package redir

import (
	"errors"
	"net"
	"syscall"
	"unsafe"

	"github.com/Dreamacro/clash/transport/socks5"
)

const (
	SO_ORIGINAL_DST      = 80 // from linux/include/uapi/linux/netfilter_ipv4.h
	IP6T_SO_ORIGINAL_DST = 80 // from linux/include/uapi/linux/netfilter_ipv6/ip6_tables.h
)

func parserPacket(conn net.Conn) (socks5.Addr, error) {
	c, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, errors.New("only work with TCP connection")
	}

	rc, err := c.SyscallConn()
	if err != nil {
		return nil, err
	}

	var addr socks5.Addr

	rc.Control(func(fd uintptr) {
		addr, err = getorigdst(fd)
	})

	return addr, err
}

// Call getorigdst() from linux/net/ipv4/netfilter/nf_conntrack_l3proto_ipv4.c
func getorigdst(fd uintptr) (socks5.Addr, error) {
	raw := syscall.RawSockaddrInet4{}
	siz := unsafe.Sizeof(raw)
	_, _, err := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.IPPROTO_IP, SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&siz)), 0)
	if err != 0 {
		return nil, err
	}

	addr := make([]byte, 1+net.IPv4len+2)
	addr[0] = socks5.AtypIPv4
	copy(addr[1:1+net.IPv4len], raw.Addr[:])
	port := (*[2]byte)(unsafe.Pointer(&raw.Port)) // big-endian
	addr[1+net.IPv4len], addr[1+net.IPv4len+1] = port[0], port[1]
	return addr, nil
}
