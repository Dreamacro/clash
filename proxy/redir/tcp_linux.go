package redir

import (
	"errors"
	"net"
	"syscall"
	"unsafe"

	"github.com/Dreamacro/clash/component/socks5"
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
		if (conn.LocalAddr().(*net.TCPAddr)).IP.To4() != nil {
			addr, err = getorigdst(fd, false)
		} else {
			addr, err = getorigdst(fd, true)
		}
	})

	return addr, err
}

// Call getorigdst() from linux/net/ipv4/netfilter/nf_conntrack_l3proto_ipv4.c
func getorigdst(fd uintptr, isIPv6 bool) (socks5.Addr, error) {
	var level uintptr = syscall.IPPROTO_IP
	var optname uintptr = SO_ORIGINAL_DST
	optval := syscall.RawSockaddrAny{}
	optlen := unsafe.Sizeof(optval)

	if isIPv6 {
		level = syscall.IPPROTO_IPV6
		optname = IP6T_SO_ORIGINAL_DST
	}

	if err := socketcall(GETSOCKOPT, fd, level, optname, uintptr(unsafe.Pointer(&optval)), uintptr(unsafe.Pointer(&optlen)), 0); err != nil {
		return nil, err
	}

	/*
	   The SOCKS request is formed as follows:
	        +----+-----+-------+------+----------+----------+
	        |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
	        +----+-----+-------+------+----------+----------+
	        | 1  |  1  | X'00' |  1   | Variable |    2     |
	        +----+-----+-------+------+----------+----------+
	*/
	if !isIPv6 {
		addr := make([]byte, 1+net.IPv4len+2)
		addr[0] = socks5.AtypIPv4
		ipv4Addr := (*syscall.RawSockaddrInet4)(unsafe.Pointer(&optval))
		copy(addr[1:1+net.IPv4len], ipv4Addr.Addr[:])
		port := (*[2]byte)(unsafe.Pointer(&ipv4Addr.Port)) // big-endian
		addr[1+net.IPv4len], addr[1+net.IPv4len+1] = port[0], port[1]
		return addr, nil
	} else {
		addr := make([]byte, 1+net.IPv6len+2)
		addr[0] = socks5.AtypIPv6
		ipv6Addr := (*syscall.RawSockaddrInet6)(unsafe.Pointer(&optval))
		copy(addr[1:1+net.IPv6len], ipv6Addr.Addr[:])
		port := (*[2]byte)(unsafe.Pointer(&ipv6Addr.Port)) // big-endian
		addr[1+net.IPv6len], addr[1+net.IPv6len+1] = port[0], port[1]
		return addr, nil
	}
}
