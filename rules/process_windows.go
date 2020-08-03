package rules

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/Dreamacro/clash/common/cache"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"

	"golang.org/x/sys/windows"
)

const (
	tcpTableFunc      = "GetExtendedTcpTable"
	tcpTablePidConn   = 4
	udpTableFunc      = "GetExtendedUdpTable"
	udpTablePid       = 1
	queryProcNameFunc = "QueryFullProcessImageNameW"
)

var (
	processCache = cache.NewLRUCache(cache.WithAge(2), cache.WithSize(64))
	errNotFound  = errors.New("process not found")
	matchMeta    = func(p *Process, m *C.Metadata) bool { return false }

	getExTcpTable uintptr
	getExUdpTable uintptr
	queryProcName uintptr

	once sync.Once
)

func initWin32API() error {
	h, err := windows.LoadLibrary("iphlpapi.dll")
	if err != nil {
		return fmt.Errorf("LoadLibrary iphlpapi.dll failed: %s", err.Error())
	}

	getExTcpTable, err = windows.GetProcAddress(h, tcpTableFunc)
	if err != nil {
		return fmt.Errorf("GetProcAddress of %s failed: %s", tcpTableFunc, err.Error())
	}

	getExUdpTable, err = windows.GetProcAddress(h, udpTableFunc)
	if err != nil {
		return fmt.Errorf("GetProcAddress of %s failed: %s", udpTableFunc, err.Error())
	}

	h, err = windows.LoadLibrary("kernel32.dll")
	if err != nil {
		return fmt.Errorf("LoadLibrary kernel32.dll failed: %s", err.Error())
	}

	queryProcName, err = windows.GetProcAddress(h, queryProcNameFunc)
	if err != nil {
		return fmt.Errorf("GetProcAddress of %s failed: %s", queryProcNameFunc, err.Error())
	}

	return nil
}

type Process struct {
	adapter string
	process string
}

func (p *Process) RuleType() C.RuleType {
	return C.Process
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

func match(p *Process, metadata *C.Metadata) bool {
	key := fmt.Sprintf("%s:%s:%s", metadata.NetWork.String(), metadata.SrcIP.String(), metadata.SrcPort)
	cached, hit := processCache.Get(key)
	if !hit {
		processName, err := resolveProcessName(metadata)
		if err != nil {
			log.Debugln("[%s] Resolve process of %s failed: %s", C.Process.String(), key, err.Error())
		}

		processCache.Set(key, processName)
		cached = processName
	}
	return strings.EqualFold(cached.(string), p.process)
}

func (p *Process) Match(metadata *C.Metadata) bool {
	return matchMeta(p, metadata)
}

func NewProcess(process string, adapter string) (*Process, error) {
	once.Do(func() {
		err := initWin32API()
		if err != nil {
			log.Errorln("Initialize PROCESS-NAME failed: %s", err.Error())
			log.Warnln("All PROCESS-NAMES rules will be skiped")
			return
		}
		matchMeta = match
	})
	return &Process{
		adapter: adapter,
		process: process,
	}, nil
}

func resolveProcessName(metadata *C.Metadata) (string, error) {
	ip := metadata.SrcIP
	family := windows.AF_INET
	if ip.To4() == nil {
		family = windows.AF_INET6
	}

	var class int
	var fn uintptr
	switch metadata.NetWork {
	case C.TCP:
		fn = getExTcpTable
		class = tcpTablePidConn
	case C.UDP:
		fn = getExUdpTable
		class = udpTablePid
	default:
		return "", ErrInvalidNetwork
	}

	srcPort, err := strconv.Atoi(metadata.SrcPort)
	if err != nil {
		return "", err
	}

	buf, err := getTransportTable(fn, family, class)
	if err != nil {
		return "", err
	}

	s := newSearcher(family == windows.AF_INET, metadata.NetWork == C.TCP)

	pid, err := s.Search(buf, ip, uint16(srcPort))
	if err != nil {
		return "", err
	}
	return getExecPathFromPID(pid)
}

type searcher struct {
	itemSize int
	port     int
	ip       int
	ipSize   int
	pid      int
	tcpState int
}

func (s *searcher) Search(b []byte, ip net.IP, port uint16) (uint32, error) {
	n := int(readNativeUint32(b[:4]))
	itemSize := s.itemSize
	for i := 0; i < n; i++ {
		row := b[4+itemSize*i : 4+itemSize*(i+1)]

		if s.tcpState >= 0 {
			tcpState := readNativeUint32(row[s.tcpState : s.tcpState+4])
			// MIB_TCP_STATE_ESTAB, only check established connections for TCP
			if tcpState != 5 {
				continue
			}
		}

		// according to MSDN, only the lower 16 bits of dwLocalPort are used and the port number is in network endian.
		// this field can be illustrated as follows depends on different machine endianess:
		//     little endian: [ MSB LSB  0   0  ]   interpret as native uint32 is ((LSB<<8)|MSB)
		//       big  endian: [  0   0  MSB LSB ]   interpret as native uint32 is ((MSB<<8)|LSB)
		// so we need an syscall.Ntohs on the lower 16 bits after read the port as native uint32
		srcPort := syscall.Ntohs(uint16(readNativeUint32(row[s.port : s.port+4])))
		if srcPort != port {
			continue
		}

		srcIP := net.IP(row[s.ip : s.ip+s.ipSize])
		if !ip.Equal(srcIP) {
			continue
		}

		pid := readNativeUint32(row[s.pid : s.pid+4])
		return pid, nil
	}
	return 0, errNotFound
}

func newSearcher(isV4, isTCP bool) *searcher {
	var itemSize, port, ip, ipSize, pid int
	tcpState := -1
	switch {
	case isV4 && isTCP:
		// struct MIB_TCPROW_OWNER_PID
		itemSize, port, ip, ipSize, pid, tcpState = 24, 8, 4, 4, 20, 0
	case isV4 && !isTCP:
		// struct MIB_UDPROW_OWNER_PID
		itemSize, port, ip, ipSize, pid = 12, 4, 0, 4, 8
	case !isV4 && isTCP:
		// struct MIB_TCP6ROW_OWNER_PID
		itemSize, port, ip, ipSize, pid, tcpState = 56, 20, 0, 16, 52, 48
	case !isV4 && !isTCP:
		// struct MIB_UDP6ROW_OWNER_PID
		itemSize, port, ip, ipSize, pid = 28, 20, 0, 16, 24
	}

	return &searcher{
		itemSize: itemSize,
		port:     port,
		ip:       ip,
		ipSize:   ipSize,
		pid:      pid,
		tcpState: tcpState,
	}
}

func getTransportTable(fn uintptr, family int, class int) ([]byte, error) {
	for size, buf := uint32(8), make([]byte, 8); ; {
		ptr := unsafe.Pointer(&buf[0])
		err, _, _ := syscall.Syscall6(fn, 6, uintptr(ptr), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(class), 0)

		switch err {
		case 0:
			return buf, nil
		case uintptr(syscall.ERROR_INSUFFICIENT_BUFFER):
			buf = make([]byte, size)
		default:
			return nil, fmt.Errorf("syscall error: %d", err)
		}
	}
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

func getExecPathFromPID(pid uint32) (string, error) {
	// kernel process starts with a colon in order to distinguish with normal processes
	switch pid {
	case 0:
		// reserved pid for system idle process
		return ":System Idle Process", nil
	case 4:
		// reserved pid for windows kernel image
		return ":System", nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, syscall.MAX_LONG_PATH)
	size := uint32(len(buf))
	r1, _, err := syscall.Syscall6(
		queryProcName, 4,
		uintptr(h),
		uintptr(1),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0, 0)
	if r1 == 0 {
		return "", err
	}
	return filepath.Base(syscall.UTF16ToString(buf[:size])), nil
}
