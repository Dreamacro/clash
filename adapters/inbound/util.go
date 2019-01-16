package adapters

import (
	"net"
	"net/http"
	"strconv"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/go-shadowsocks2/socks"
)

func parseSocksAddr(target socks.Addr) *C.Metadata {
	metadata := &C.Metadata{
		NetWork:  C.TCP,
		AddrType: int(target[0]),
	}

	switch target[0] {
	case socks.AtypDomainName:
		metadata.Host = string(target[2 : 2+target[1]])
		metadata.Port = strconv.Itoa((int(target[2+target[1]]) << 8) | int(target[2+target[1]+1]))
	case socks.AtypIPv4:
		ip := net.IP(target[1 : 1+net.IPv4len])
		metadata.IP = &ip
		metadata.Port = strconv.Itoa((int(target[1+net.IPv4len]) << 8) | int(target[1+net.IPv4len+1]))
	case socks.AtypIPv6:
		ip := net.IP(target[1 : 1+net.IPv6len])
		metadata.IP = &ip
		metadata.Port = strconv.Itoa((int(target[1+net.IPv6len]) << 8) | int(target[1+net.IPv6len+1]))
	}

	return metadata
}

func parseHTTPAddr(request *http.Request) *C.Metadata {
	host := request.URL.Hostname()
	port := request.URL.Port()
	if port == "" {
		port = "80"
	}

	metadata := &C.Metadata{
		NetWork:  C.TCP,
		Source:   C.HTTP,
		AddrType: C.AtypDomainName,
		Host:     host,
		IP:       nil,
		Port:     port,
	}

	ip := net.ParseIP(host)
	if ip != nil {
		switch {
		case ip.To4() == nil:
			metadata.AddrType = C.AtypIPv6
		default:
			metadata.AddrType = C.AtypIPv4
		}
		metadata.IP = &ip
	}

	return metadata
}

func parseSourceIP(conn net.Conn) *net.IP {
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return &addr.IP
	}
	return nil
}
