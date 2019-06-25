package socks5

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strconv"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

// Error represents a SOCKS error
type Error byte

func (err Error) Error() string {
	return "SOCKS error: " + strconv.Itoa(int(err))
}

// Command is request commands as defined in RFC 1928 section 4.
type Command = uint8

// SOCKS request commands as defined in RFC 1928 section 4.
const (
	CmdConnect      Command = 1
	CmdBind         Command = 2
	CmdUDPAssociate Command = 3
)

// SOCKS address types as defined in RFC 1928 section 5.
const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4
)

// MaxAddrLen is the maximum size of SOCKS address in bytes.
const MaxAddrLen = 1 + 1 + 255 + 2

// Control the max length of auth data.
const MaxAuthLen = 512

// Addr represents a SOCKS address as defined in RFC 1928 section 5.
type Addr = []byte

// SOCKS errors as defined in RFC 1928 section 6.
const (
	ErrGeneralFailure       = Error(1)
	ErrConnectionNotAllowed = Error(2)
	ErrNetworkUnreachable   = Error(3)
	ErrHostUnreachable      = Error(4)
	ErrConnectionRefused    = Error(5)
	ErrTTLExpired           = Error(6)
	ErrCommandNotSupported  = Error(7)
	ErrAddressNotSupported  = Error(8)
)

type User struct {
	Username string
	Password string
}

// ServerHandshake fast-tracks SOCKS initialization to get target address to connect on server side.
func ServerHandshake(rw net.Conn, authenticator C.Authenticator) (addr Addr, command Command, err error) {
	// Read RFC 1928 for request and reply structure and sizes.
	buf := make([]byte, MaxAddrLen)
	// read VER, NMETHODS, METHODS
	if _, err = io.ReadFull(rw, buf[:2]); err != nil {
		return
	}
	nmethods := buf[1]
	if _, err = io.ReadFull(rw, buf[:nmethods]); err != nil {
		return
	}
	// write VER METHOD
	if authenticator.Enabled() {
		if _, err = rw.Write([]byte{5, 2}); err != nil {
			return
		}

		// Get header
		header := []byte{0, 0}
		if _, err = io.ReadAtLeast(rw, header, 2); err != nil {
			return
		}

		// Get username
		userLen := int(header[1])
		if userLen <= 0 || userLen >= MaxAuthLen {
			rw.Write([]byte{1, 1})
			err = ErrGeneralFailure
			return
		}
		user := make([]byte, userLen)
		if _, err = io.ReadAtLeast(rw, user, userLen); err != nil {
			return
		}

		// Get password
		if _, err = rw.Read(header[:1]); err != nil {
			return
		}
		passLen := int(header[0])
		if passLen <= 0 || passLen >= MaxAuthLen {
			rw.Write([]byte{1, 1})
			err = ErrGeneralFailure
			return
		}
		pass := make([]byte, passLen)
		if _, err = io.ReadAtLeast(rw, pass, passLen); err != nil {
			return
		}

		// Verify
		ok := authenticator.Verify(string(user), string(pass))
		if !ok {
			log.Infoln("Auth failed from %s", rw.RemoteAddr().String())
			rw.Write([]byte{1, 1})
			err = ErrGeneralFailure
			return
		}

		// Response auth state
		_, err = rw.Write([]byte{1, 0})
		if err != nil {
			return
		}

	} else {
		if _, err = rw.Write([]byte{5, 0}); err != nil {
			return
		}
	}
	// read VER CMD RSV ATYP DST.ADDR DST.PORT
	if _, err = io.ReadFull(rw, buf[:3]); err != nil {
		return
	}

	if buf[1] != CmdConnect && buf[1] != CmdUDPAssociate {
		err = ErrCommandNotSupported
		return
	}

	command = buf[1]
	addr, err = readAddr(rw, buf)
	if err != nil {
		return
	}
	// write VER REP RSV ATYP BND.ADDR BND.PORT
	_, err = rw.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	return
}

// ClientHandshake fast-tracks SOCKS initialization to get target address to connect on client side.
func ClientHandshake(rw io.ReadWriter, addr Addr, cammand Command, user *User) error {
	buf := make([]byte, MaxAddrLen)
	var err error

	// VER, NMETHODS, METHODS
	if user != nil {
		_, err = rw.Write([]byte{5, 1, 2})
	} else {
		_, err = rw.Write([]byte{5, 1, 0})
	}
	if err != nil {
		return err
	}

	// VER, METHOD
	if _, err := io.ReadFull(rw, buf[:2]); err != nil {
		return err
	}

	if buf[0] != 5 {
		return errors.New("SOCKS version error")
	}

	if buf[1] == 2 {
		// password protocol version
		authMsg := &bytes.Buffer{}
		authMsg.WriteByte(1)
		authMsg.WriteByte(uint8(len(user.Username)))
		authMsg.WriteString(user.Username)
		authMsg.WriteByte(uint8(len(user.Password)))
		authMsg.WriteString(user.Password)

		if _, err := rw.Write(authMsg.Bytes()); err != nil {
			return err
		}

		if _, err := io.ReadFull(rw, buf[:2]); err != nil {
			return err
		}

		if buf[1] != 0 {
			return errors.New("rejected username/password")
		}
	} else if buf[1] != 0 {
		return errors.New("SOCKS need auth")
	}

	// VER, CMD, RSV, ADDR
	if _, err := rw.Write(bytes.Join([][]byte{{5, cammand, 0}, addr}, []byte(""))); err != nil {
		return err
	}

	if _, err := io.ReadFull(rw, buf[:10]); err != nil {
		return err
	}

	return nil
}

func readAddr(r io.Reader, b []byte) (Addr, error) {
	if len(b) < MaxAddrLen {
		return nil, io.ErrShortBuffer
	}
	_, err := io.ReadFull(r, b[:1]) // read 1st byte for address type
	if err != nil {
		return nil, err
	}

	switch b[0] {
	case AtypDomainName:
		_, err = io.ReadFull(r, b[1:2]) // read 2nd byte for domain length
		if err != nil {
			return nil, err
		}
		domainLength := uint16(b[1])
		_, err = io.ReadFull(r, b[2:2+domainLength+2])
		return b[:1+1+domainLength+2], err
	case AtypIPv4:
		_, err = io.ReadFull(r, b[1:1+net.IPv4len+2])
		return b[:1+net.IPv4len+2], err
	case AtypIPv6:
		_, err = io.ReadFull(r, b[1:1+net.IPv6len+2])
		return b[:1+net.IPv6len+2], err
	}

	return nil, ErrAddressNotSupported
}

// SplitAddr slices a SOCKS address from beginning of b. Returns nil if failed.
func SplitAddr(b []byte) Addr {
	addrLen := 1
	if len(b) < addrLen {
		return nil
	}

	switch b[0] {
	case AtypDomainName:
		if len(b) < 2 {
			return nil
		}
		addrLen = 1 + 1 + int(b[1]) + 2
	case AtypIPv4:
		addrLen = 1 + net.IPv4len + 2
	case AtypIPv6:
		addrLen = 1 + net.IPv6len + 2
	default:
		return nil

	}

	if len(b) < addrLen {
		return nil
	}

	return b[:addrLen]
}

// ParseAddr parses the address in string s. Returns nil if failed.
func ParseAddr(s string) Addr {
	var addr Addr
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			addr = make([]byte, 1+net.IPv4len+2)
			addr[0] = AtypIPv4
			copy(addr[1:], ip4)
		} else {
			addr = make([]byte, 1+net.IPv6len+2)
			addr[0] = AtypIPv6
			copy(addr[1:], ip)
		}
	} else {
		if len(host) > 255 {
			return nil
		}
		addr = make([]byte, 1+1+len(host)+2)
		addr[0] = AtypDomainName
		addr[1] = byte(len(host))
		copy(addr[2:], host)
	}

	portnum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil
	}

	addr[len(addr)-2], addr[len(addr)-1] = byte(portnum>>8), byte(portnum)

	return addr
}
