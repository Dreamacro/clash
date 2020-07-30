package rules

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Dreamacro/clash/common/cache"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

// store process name for when dealing with multiple PROCESS-NAME rules
var processCache = cache.NewLRUCache(cache.WithAge(2), cache.WithSize(64))

type Process struct {
	adapter string
	process string
}

func (ps *Process) RuleType() C.RuleType {
	return C.Process
}

func (ps *Process) Match(metadata *C.Metadata) bool {
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

	return strings.EqualFold(cached.(string), ps.process)
}

func (p *Process) Adapter() string {
	return p.adapter
}

func (p *Process) Payload() string {
	return p.process
}

func (p *Process) ShouldResolveIP() bool {
	return false
}

func NewProcess(process string, adapter string) (*Process, error) {
	return &Process{
		adapter: adapter,
		process: process,
	}, nil
}

const (
	procpidpathinfo     = 0xb
	procpidpathinfosize = 1024
	proccallnumpidinfo  = 0x2
)

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
	firstZero := bytes.IndexByte(buf, 0)
	if firstZero <= 0 {
		return "", nil
	}

	return filepath.Base(string(buf[:firstZero])), nil
}

func getExecPathFromAddress(metadata *C.Metadata) (string, error) {
	ip := metadata.SrcIP
	port, err := strconv.Atoi(metadata.SrcPort)
	if err != nil {
		return "", err
	}

	var spath string
	switch metadata.NetWork {
	case C.TCP:
		spath = "net.inet.tcp.pcblist_n"
	case C.UDP:
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
	if metadata.NetWork == C.TCP {
		// rup8(sizeof(xtcpcb_n))
		itemSize += 208
	}
	// skip the first and last xinpgen(24 bytes) block
	for i := 24; i < len(buf)-24; i += itemSize {
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

	return "", errors.New("process not found")
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}
