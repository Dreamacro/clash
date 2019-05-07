package hijack

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// HTTP is shadowsocks http simple-obfs implementation
type Hijack struct {
	net.Conn
	first bool
	addr  string
	auth  string
	buf   *bytes.Buffer
}

func parseFirstLine(line string) (r1, r2, r3 string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}

func (h *Hijack) Write(b []byte) (int, error) {
	if h.first {
		str := string(b)
		if i := strings.Index(str, "\r\n"); i > -1 {
			method, uri, protocol, isHttp := parseFirstLine(str[:i])
			if isHttp {
				h.buf.WriteString(fmt.Sprintf("%s http://%s%s %s", method, h.addr, uri, protocol))
				if h.auth != "" {
					h.buf.WriteString("\r\n")
					h.buf.WriteString("proxy-auth: Basic" + h.auth)
				}
				h.buf.WriteString(str[i:])
				_, err := h.Conn.Write(h.buf.Bytes())
				h.first = false
				return len(b), err
			}
		}
		return 0, io.EOF
	}
	return h.Conn.Write(b)
}

func NewHijack(conn net.Conn, addr string, auth string) net.Conn {
	return &Hijack{
		Conn:  conn,
		addr:  addr,
		auth:  auth,
		first: true,
		buf:   bytes.NewBuffer(nil),
	}
}
