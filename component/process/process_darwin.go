package process

import (
	"encoding/binary"
	"net"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	procpidpathinfo     = 0xb
	procpidpathinfosize = 1024
	proccallnumpidinfo  = 0x2
)

func findProcessName(network string, ip net.IP, port int) (string, error) {
	var spath string
	switch network {
	case TCP:
		spath = "net.inet.tcp.pcblist_n"
	case UDP:
		spath = "net.inet.udp.pcblist_n"
	default:
		return "", ErrInvalidNetwork
	}

	isIPv4 := ip.To4() != nil

	value, err := syscall.Sysctl(spath)
	if err != nil {
		return "", err
	}

	buf := []byte(value)

	// from darwin-xnu/bsd/netinet/in_pcblist.c:get_pcblist_n
	// size/offset are round up (aligned) to 8 bytes in darwin
	// rup8(sizeof(xinpcb_n)) + rup8(sizeof(xsocket_n)) +
	// 2 * rup8(sizeof(xsockbuf_n)) + rup8(sizeof(xsockstat_n))
	itemSize := 384
	if network == TCP {
		// rup8(sizeof(xtcpcb_n))
		itemSize += 208
	}
	// skip the first xinpgen(24 bytes) block
	for i := 24; i+itemSize <= len(buf); i += itemSize {
		// offset of xinpcb_n and xsocket_n
		inp, so := i, i+104

		srcPort := binary.BigEndian.Uint16(buf[inp+18 : inp+20])
		if uint16(port) != srcPort {
			continue
		}

		// xinpcb_n.inp_vflag
		flag := buf[inp+44]

		var srcIP net.IP
		switch {
		case flag&0x1 > 0 && isIPv4:
			// ipv4
			srcIP = net.IP(buf[inp+76 : inp+80])
		case flag&0x2 > 0 && !isIPv4:
			// ipv6
			srcIP = net.IP(buf[inp+64 : inp+80])
		default:
			continue
		}

		if !ip.Equal(srcIP) {
			continue
		}

		// xsocket_n.so_last_pid
		pid := readNativeUint32(buf[so+68 : so+72])
		return getExecPathFromPID(pid)
	}

	return "", ErrNotFound
}

func getExecPathFromPID(pid uint32) (string, error) {
	buf := make([]byte, procpidpathinfosize)
	_, _, errno := syscall.Syscall6(
		syscall.SYS_PROC_INFO,
		proccallnumpidinfo,
		uintptr(pid),
		procpidpathinfo,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		procpidpathinfosize)
	if errno != 0 {
		return "", errno
	}

	return filepath.Base(unix.ByteSliceToString(buf)), nil
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}
