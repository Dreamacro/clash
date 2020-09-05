package rules

import (
	"encoding/binary"
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
)

// store process name for when dealing with multiple PROCESS-NAME rules
var (
	processCache = cache.NewLRUCache(cache.WithAge(2), cache.WithSize(64))
	errNotFound  = errors.New("process not found")
	matchMeta    = func(p *Process, m *C.Metadata) bool { return false }

	defaultSearcher *searcher

	once sync.Once
)

type Process struct {
	adapter string
	process string
}

func (ps *Process) RuleType() C.RuleType {
	return C.Process
}

func match(ps *Process, metadata *C.Metadata) bool {
	key := fmt.Sprintf("%s:%s:%s", metadata.NetWork.String(), metadata.SrcIP.String(), metadata.SrcPort)
	cached, hit := processCache.Get(key)
	if !hit {
		name, err := getExecPathFromAddress(metadata)
		if err != nil {
			log.Debugln("[%s] getExecPathFromAddress error: %s", C.Process.String(), err.Error())
		}

		processCache.Set(key, name)

		cached = name
	}

	return strings.EqualFold(cached.(string), ps.process)
}

func (ps *Process) Match(metadata *C.Metadata) bool {
	return matchMeta(ps, metadata)
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
	once.Do(func() {
		err := initSearcher()
		if err != nil {
			log.Errorln("Initialize PROCESS-NAME failed: %s", err.Error())
			log.Warnln("All PROCESS-NAME rules will be skipped")
			return
		}
		matchMeta = match
	})
	return &Process{
		adapter: adapter,
		process: process,
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

	return filepath.Base(string(buf[:size-1])), nil
}

func getExecPathFromAddress(metadata *C.Metadata) (string, error) {
	ip := metadata.SrcIP
	port, err := strconv.Atoi(metadata.SrcPort)
	if err != nil {
		return "", err
	}

	var spath string
	var isTCP bool
	switch metadata.NetWork {
	case C.TCP:
		spath = "net.inet.tcp.pcblist"
		isTCP = true
	case C.UDP:
		spath = "net.inet.udp.pcblist"
		isTCP = false
	default:
		return "", ErrInvalidNetwork
	}

	value, err := syscall.Sysctl(spath)
	if err != nil {
		return "", err
	}

	buf := []byte(value)

	pid, err := defaultSearcher.Search(buf, ip, uint16(port), isTCP)
	if err != nil {
		return "", err
	}

	return getExecPathFromPID(pid)
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

type searcher struct {
	// sizeof(struct xinpgen)
	headSize int
	// sizeof(struct xtcpcb)
	tcpItemSize int
	// sizeof(struct xinpcb)
	udpItemSize  int
	udpInpOffset int
	port         int
	ip           int
	vflag        int
	socket       int

	// sizeof(struct xfile)
	fileItemSize int
	data         int
	pid          int
}

func (s *searcher) Search(buf []byte, ip net.IP, port uint16, isTCP bool) (uint32, error) {
	var itemSize int
	var inpOffset int

	if isTCP {
		// struct xtcpcb
		itemSize = s.tcpItemSize
		inpOffset = 8
	} else {
		// struct xinpcb
		itemSize = s.udpItemSize
		inpOffset = s.udpInpOffset
	}

	isIPv4 := ip.To4() != nil
	// skip the first xinpgen block
	for i := s.headSize; i+itemSize <= len(buf); i += itemSize {
		inp := i + inpOffset

		srcPort := binary.BigEndian.Uint16(buf[inp+s.port : inp+s.port+2])

		if port != srcPort {
			continue
		}

		// xinpcb.inp_vflag
		flag := buf[inp+s.vflag]

		var srcIP net.IP
		switch {
		case flag&0x1 > 0 && isIPv4:
			// ipv4
			srcIP = net.IP(buf[inp+s.ip : inp+s.ip+4])
		case flag&0x2 > 0 && !isIPv4:
			// ipv6
			srcIP = net.IP(buf[inp+s.ip-12 : inp+s.ip+4])
		default:
			continue
		}

		if !ip.Equal(srcIP) {
			continue
		}

		// xsocket.xso_so, interpreted as big endian anyway since it's only used for comparison
		socket := binary.BigEndian.Uint64(buf[inp+s.socket : inp+s.socket+8])
		return s.searchSocketPid(socket)
	}
	return 0, errNotFound
}

func (s *searcher) searchSocketPid(socket uint64) (uint32, error) {
	value, err := syscall.Sysctl("kern.file")
	if err != nil {
		return 0, err
	}

	buf := []byte(value)

	// struct xfile
	itemSize := s.fileItemSize
	for i := 0; i+itemSize <= len(buf); i += itemSize {
		// xfile.xf_data
		data := binary.BigEndian.Uint64(buf[i+s.data : i+s.data+8])
		if data == socket {
			// xfile.xf_pid
			pid := readNativeUint32(buf[i+s.pid : i+s.pid+4])
			return pid, nil
		}
	}
	return 0, errNotFound
}

func newSearcher(major int) *searcher {
	var s *searcher = nil
	switch major {
	case 11:
		s = &searcher{
			headSize:     32,
			tcpItemSize:  1304,
			udpItemSize:  632,
			port:         198,
			ip:           228,
			vflag:        116,
			socket:       88,
			fileItemSize: 80,
			data:         56,
			pid:          8,
			udpInpOffset: 8,
		}
	case 12:
		s = &searcher{
			headSize:     64,
			tcpItemSize:  744,
			udpItemSize:  400,
			port:         254,
			ip:           284,
			vflag:        392,
			socket:       16,
			fileItemSize: 128,
			data:         56,
			pid:          8,
		}
	}
	return s
}

func initSearcher() error {
	osRelease, err := syscall.Sysctl("kern.osrelease")
	if err != nil {
		return err
	}

	dot := strings.Index(osRelease, ".")
	if dot != -1 {
		osRelease = osRelease[:dot]
	}
	major, err := strconv.Atoi(osRelease)
	if err != nil {
		return err
	}
	defaultSearcher = newSearcher(major)
	if defaultSearcher == nil {
		return fmt.Errorf("unsupported freebsd version %d", major)
	}
	return nil
}
