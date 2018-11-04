package adapters

import (
	"bytes"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/simple-obfs"
	C "github.com/Dreamacro/clash/constant"

	"github.com/Dreamacro/go-shadowsocks2/core"
	"github.com/Dreamacro/go-shadowsocks2/socks"
)

// ShadowsocksAdapter is a shadowsocks adapter
type ShadowsocksAdapter struct {
	conn net.Conn
}

// Close is used to close connection
func (ss *ShadowsocksAdapter) Close() {
	ss.conn.Close()
}

func (ss *ShadowsocksAdapter) Conn() net.Conn {
	return ss.conn
}

type ShadowSocks struct {
	server   string
	name     string
	obfs     string
	obfsHost string
	cipher   core.Cipher
}

type ShadowSocksOption struct {
	Name     string `proxy:"name"`
	Server   string `proxy:"server"`
	Port     int    `proxy:"port"`
	Password string `proxy:"password"`
	Cipher   string `proxy:"cipher"`
	Obfs     string `proxy:"obfs,omitempty"`
	ObfsHost string `proxy:"obfs-host,omitempty"`
}

func (ss *ShadowSocks) Name() string {
	return ss.name
}

func (ss *ShadowSocks) Type() C.AdapterType {
	return C.Shadowsocks
}

func (ss *ShadowSocks) Generator(metadata *C.Metadata) (adapter C.ProxyAdapter, err error) {
	c, err := net.DialTimeout("tcp", ss.server, tcpTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s connect error", ss.server)
	}
	tcpKeepAlive(c)
	switch ss.obfs {
	case "tls":
		c = obfs.NewTLSObfs(c, ss.obfsHost)
	case "http":
		_, port, _ := net.SplitHostPort(ss.server)
		c = obfs.NewHTTPObfs(c, ss.obfsHost, port)
	}
	c = ss.cipher.StreamConn(c)
	_, err = c.Write(serializesSocksAddr(metadata))
	return &ShadowsocksAdapter{conn: c}, err
}

func NewShadowSocks(option ShadowSocksOption) (*ShadowSocks, error) {
	server := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	ciph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ss %s initialize error: %s", server, err.Error())
	}

	obfs := option.Obfs
	obfsHost := "bing.com"
	if option.ObfsHost != "" {
		obfsHost = option.ObfsHost
	}

	return &ShadowSocks{
		server:   server,
		name:     option.Name,
		cipher:   ciph,
		obfs:     obfs,
		obfsHost: obfsHost,
	}, nil
}

func serializesSocksAddr(metadata *C.Metadata) []byte {
	var buf [][]byte
	aType := uint8(metadata.AddrType)
	p, _ := strconv.Atoi(metadata.Port)
	port := []byte{uint8(p >> 8), uint8(p & 0xff)}
	switch metadata.AddrType {
	case socks.AtypDomainName:
		len := uint8(len(metadata.Host))
		host := []byte(metadata.Host)
		buf = [][]byte{{aType, len}, host, port}
	case socks.AtypIPv4:
		host := metadata.IP.To4()
		buf = [][]byte{{aType}, host, port}
	case socks.AtypIPv6:
		host := metadata.IP.To16()
		buf = [][]byte{{aType}, host, port}
	}
	return bytes.Join(buf, nil)
}
