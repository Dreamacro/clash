package rules

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Dreamacro/clash/common/pool"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

// from https://github.com/vishvananda/netlink/blob/bca67dfc8220b44ef582c9da4e9172bf1c9ec973/nl/nl_linux.go#L52-L62
func init() {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		nativeEndian = binary.BigEndian
	} else {
		nativeEndian = binary.LittleEndian
	}
}

type SocketResolver func(metadata *C.Metadata) (inode, uid int, err error)
type ProcessNameResolver func(inode, uid int) (name string, err error)

// export for android
var (
	DefaultSocketResolver      SocketResolver      = resolveSocketByNetlink
	DefaultProcessNameResolver ProcessNameResolver = resolveProcessNameByProcSearch
)

func (p *Process) Match(metadata *C.Metadata) bool {
	key := fmt.Sprintf("%s:%s:%s", metadata.NetWork.String(), metadata.SrcIP.String(), metadata.SrcPort)
	cached, hit := processCache.Get(key)
	if !hit {
		processName, err := resolveProcessName(metadata)
		if err != nil {
			log.Debugln("[%s] Resolve process of %s failure: %s", C.Process.String(), key, err.Error())
		}

		processCache.Set(key, processName)

		cached = processName
	}

	processName := cached.(string)
	if !p.fullMatch {
		processName = filepath.Base(processName)
	}

	return strings.EqualFold(processName, p.process)
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

const (
	sizeOfSocketDiagRequest = syscall.SizeofNlMsghdr + 8 + 48
	socketDiagByFamily      = 20
	pathProc                = "/proc"
)

var nativeEndian binary.ByteOrder = binary.LittleEndian

func resolveProcessName(metadata *C.Metadata) (string, error) {
	inode, uid, err := DefaultSocketResolver(metadata)
	if err != nil {
		return "", err
	}

	return DefaultProcessNameResolver(inode, uid)
}

func resolveSocketByNetlink(metadata *C.Metadata) (int, int, error) {
	var family byte
	var protocol byte

	switch metadata.NetWork {
	case C.TCP:
		protocol = syscall.IPPROTO_TCP
	case C.UDP:
		protocol = syscall.IPPROTO_UDP
	default:
		return 0, 0, ErrInvalidNetwork
	}

	if metadata.SrcIP.To4() != nil {
		family = syscall.AF_INET
	} else {
		family = syscall.AF_INET6
	}

	srcPort, err := strconv.Atoi(metadata.SrcPort)
	if err != nil {
		return 0, 0, err
	}

	req := packSocketDiagRequest(family, protocol, metadata.SrcIP, uint16(srcPort))

	socket, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_INET_DIAG)
	if err != nil {
		return 0, 0, err
	}
	defer syscall.Close(socket)

	syscall.SetNonblock(socket, true)
	syscall.SetsockoptTimeval(socket, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &syscall.Timeval{Usec: 50})
	syscall.SetsockoptTimeval(socket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Usec: 50})

	if err := syscall.Connect(socket, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pad:    0,
		Pid:    0,
		Groups: 0,
	}); err != nil {
		return 0, 0, err
	}

	if _, err := syscall.Write(socket, req); err != nil {
		return 0, 0, err
	}

	rb := pool.Get(pool.RelayBufferSize)
	defer pool.Put(rb)

	n, err := syscall.Read(socket, rb)
	if err != nil {
		return 0, 0, err
	}

	messages, err := syscall.ParseNetlinkMessage(rb[:n])
	if err != nil {
		return 0, 0, err
	} else if len(messages) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}

	message := messages[0]
	if message.Header.Type&syscall.NLMSG_ERROR != 0 {
		return 0, 0, syscall.ESRCH
	}

	uid, inode := unpackSocketDiagResponse(&messages[0])

	return int(uid), int(inode), nil
}

func packSocketDiagRequest(family, protocol byte, source net.IP, sourcePort uint16) []byte {
	s := make([]byte, 16)

	if v4 := source.To4(); v4 != nil {
		copy(s, v4)
	} else {
		copy(s, source)
	}

	buf := make([]byte, sizeOfSocketDiagRequest)

	nativeEndian.PutUint32(buf[0:4], sizeOfSocketDiagRequest)
	nativeEndian.PutUint16(buf[4:6], socketDiagByFamily)
	nativeEndian.PutUint16(buf[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_DUMP)
	nativeEndian.PutUint32(buf[8:12], 0)
	nativeEndian.PutUint32(buf[12:16], 0)

	buf[16] = family
	buf[17] = protocol
	buf[18] = 0
	buf[19] = 0
	nativeEndian.PutUint32(buf[20:24], 0xFFFFFFFF)

	binary.BigEndian.PutUint16(buf[24:26], sourcePort)
	binary.BigEndian.PutUint16(buf[26:28], 0)

	copy(buf[28:44], s)
	copy(buf[44:60], net.IPv6zero)

	nativeEndian.PutUint32(buf[60:64], 0)
	nativeEndian.PutUint64(buf[64:72], 0xFFFFFFFFFFFFFFFF)

	return buf
}

func unpackSocketDiagResponse(msg *syscall.NetlinkMessage) (inode, uid uint32) {
	if len(msg.Data) < 72 {
		return 0, 0
	}

	data := msg.Data

	uid = nativeEndian.Uint32(data[64:68])
	inode = nativeEndian.Uint32(data[68:72])

	return
}

func resolveProcessNameByProcSearch(inode, uid int) (string, error) {
	files, err := ioutil.ReadDir(pathProc)
	if err != nil {
		return "", err
	}

	buffer := make([]byte, syscall.PathMax)
	socket := []byte(fmt.Sprintf("socket:[%d]", inode))

	for _, f := range files {
		if !f.IsDir() || !isPid(f.Name()) {
			continue
		}

		if f.Sys().(*syscall.Stat_t).Uid != uint32(uid) {
			continue
		}

		processPath := path.Join(pathProc, f.Name())
		fdPath := path.Join(processPath, "fd")

		fds, err := ioutil.ReadDir(fdPath)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			n, err := syscall.Readlink(path.Join(fdPath, fd.Name()), buffer)
			if err != nil {
				continue
			}

			if bytes.Compare(buffer[:n], socket) == 0 {
				cmdline, err := ioutil.ReadFile(path.Join(processPath, "cmdline"))
				if err != nil {
					return "", err
				}

				return splitCmdline(cmdline), nil
			}
		}
	}

	return "", syscall.ESRCH
}

func splitCmdline(cmdline []byte) string {
	indexOfEndOfString := len(cmdline)

	for i, c := range cmdline {
		if c == 0 {
			indexOfEndOfString = i
			break
		}
	}

	return string(cmdline[:indexOfEndOfString])
}

func isPid(s string) bool {
	for _, s := range s {
		if s < '0' || s > '9' {
			return false
		}
	}

	return true
}
