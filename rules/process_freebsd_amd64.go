package rules

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

func (p *Process) Match(metadata *C.Metadata) bool {
	key := fmt.Sprintf("%s:%s:%s", metadata.NetWork.String(), metadata.SrcIP.String(), metadata.SrcPort)
	cached, hit := processCache.Get(key)
	if !hit {
		name, err := getExecPathFromAddress(metadata)
		if err != nil {
			log.Debugln("[%s] getExecPathFromAddress error: %s", C.Process.String(), err.Error())
			return false
		}

		processCache.Set(key, name)

		cached = name
	}

	name := cached.(string)
	if !p.fullMatch {
		name = filepath.Base(name)
	}

	return strings.EqualFold(name, p.process)
}

func NewProcess(process string, adapter string, fullMatch bool) (*Process, error) {
	if !fullMatch {
		process = filepath.Base(process)
	}

	return &Process{
		adapter:   adapter,
		process:   process,
		fullMatch: fullMatch,
	}, nil
}

func getExecPathFromPID(pid uint32) (string, error) {
	buf := make([]byte, 2048)
	size := uint64(len(buf))
	// CTL_KERN, KERN_PROC, KERN_PROC_PATHNAME, pid
	mib := [4]uint32{1, 14, 12, pid}

	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0)
	if errno != 0 || size == 0 {
		return "", errno
	}

	return string(buf[:size-1]), nil
}

func searchSocketPid(socket uint64) (uint32, error) {
	value, err := syscall.Sysctl("kern.file")
	if err != nil {
		return 0, err
	}

	buf := []byte(value)

	// struct xfile
	itemSize := 128
	for i := 0; i < len(buf); i += itemSize {
		// xfile.xf_data
		data := binary.BigEndian.Uint64(buf[i+56 : i+64])
		if data == socket {
			// xfile.xf_pid
			pid := readNativeUint32(buf[i+8 : i+12])
			return pid, nil
		}
	}
	return 0, errors.New("pid not found")
}

func getExecPathFromAddress(metadata *C.Metadata) (string, error) {
	ip := metadata.SrcIP
	port, err := strconv.Atoi(metadata.SrcPort)
	if err != nil {
		return "", err
	}

	var spath string
	var itemSize int
	var inpOffset int
	switch metadata.NetWork {
	case C.TCP:
		spath = "net.inet.tcp.pcblist"
		// struct xtcpcb
		itemSize = 744
		inpOffset = 8
	case C.UDP:
		spath = "net.inet.udp.pcblist"
		// struct xinpcb
		itemSize = 400
		inpOffset = 0
	default:
		return "", ErrInvalidNetwork
	}

	isIPv4 := ip.To4() != nil

	value, err := syscall.Sysctl(spath)
	if err != nil {
		return "", err
	}

	buf := []byte(value)

	// skip the first and last xinpgen(64 bytes) block
	for i := 64; i < len(buf)-64; i += itemSize {
		inp := i + inpOffset

		srcPort := binary.BigEndian.Uint16(buf[inp+254 : inp+256])

		if uint16(port) != srcPort {
			continue
		}

		// xinpcb.inp_vflag
		flag := buf[inp+392]

		var srcIP net.IP
		switch {
		case flag&0x1 > 0 && isIPv4:
			// ipv4
			srcIP = net.IP(buf[inp+284 : inp+288])
		case flag&0x2 > 0 && !isIPv4:
			// ipv6
			srcIP = net.IP(buf[inp+272 : inp+288])
		default:
			continue
		}

		if !ip.Equal(srcIP) {
			continue
		}

		// xsocket.xso_so, interpreted as big endian anyway since it's only used for comparison
		socket := binary.BigEndian.Uint64(buf[inp+16 : inp+24])
		pid, err := searchSocketPid(socket)
		if err != nil {
			return "", err
		}
		return getExecPathFromPID(pid)
	}

	return "", errors.New("process not found")
}
