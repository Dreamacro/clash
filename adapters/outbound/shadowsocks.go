package adapters

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/common/structure"
	obfs "github.com/Dreamacro/clash/component/simple-obfs"
	v2rayObfs "github.com/Dreamacro/clash/component/v2ray-plugin"
	C "github.com/Dreamacro/clash/constant"

	"github.com/Dreamacro/go-shadowsocks2/core"
	"github.com/Dreamacro/go-shadowsocks2/socks"
)

type ShadowSocks struct {
	*Base
	server string
	cipher core.Cipher

	// obfs
	obfsMode   string
	obfsOption *simpleObfsOption
	wsOption   *v2rayObfs.WebsocketOption
}

type ShadowSocksOption struct {
	Name       string                 `proxy:"name"`
	Server     string                 `proxy:"server"`
	Port       int                    `proxy:"port"`
	Password   string                 `proxy:"password"`
	Cipher     string                 `proxy:"cipher"`
	Plugin     string                 `proxy:"plugin,omitempty"`
	PluginOpts map[string]interface{} `proxy:"plugin-opts,omitempty"`

	// deprecated when bump to 1.0
	Obfs     string `proxy:"obfs,omitempty"`
	ObfsHost string `proxy:"obfs-host,omitempty"`
}

type simpleObfsOption struct {
	Mode string `obfs:"mode"`
	Host string `obfs:"host,omitempty"`
}

type v2rayObfsOption struct {
	Mode           string            `obfs:"mode"`
	Host           string            `obfs:"host,omitempty"`
	Path           string            `obfs:"path,omitempty"`
	TLS            bool              `obfs:"tls,omitempty"`
	Headers        map[string]string `obfs:"headers,omitempty"`
	SkipCertVerify bool              `obfs:"skip-cert-verify,omitempty"`
}

func (ss *ShadowSocks) Generator(metadata *C.Metadata) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", ss.server, tcpTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %s", ss.server, err.Error())
	}
	tcpKeepAlive(c)
	switch ss.obfsMode {
	case "tls":
		c = obfs.NewTLSObfs(c, ss.obfsOption.Host)
	case "http":
		_, port, _ := net.SplitHostPort(ss.server)
		c = obfs.NewHTTPObfs(c, ss.obfsOption.Host, port)
	case "websocket":
		var err error
		c, err = v2rayObfs.NewWebsocketObfs(c, ss.wsOption)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %s", ss.server, err.Error())
		}
	}
	c = ss.cipher.StreamConn(c)
	_, err = c.Write(serializesSocksAddr(metadata))
	return c, err
}

func (ss *ShadowSocks) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": ss.Type().String(),
	})
}

func NewShadowSocks(option ShadowSocksOption) (*ShadowSocks, error) {
	server := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	ciph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ss %s initialize error: %s", server, err.Error())
	}

	var wsOption *v2rayObfs.WebsocketOption
	var obfsOption *simpleObfsOption
	obfsMode := ""

	// forward compatibility before 1.0
	if option.Obfs != "" {
		obfsMode = option.Obfs
		obfsOption = &simpleObfsOption{
			Host: "bing.com",
		}
		if option.ObfsHost != "" {
			obfsOption.Host = option.ObfsHost
		}
	}

	decoder := structure.NewDecoder(structure.Option{TagName: "obfs", WeaklyTypedInput: true})
	if option.Plugin == "obfs" {
		opts := simpleObfsOption{Host: "bing.com"}
		if err := decoder.Decode(option.PluginOpts, &opts); err != nil {
			return nil, fmt.Errorf("ss %s initialize obfs error: %s", server, err.Error())
		}
		obfsMode = opts.Mode
		obfsOption = &opts
	} else if option.Plugin == "v2ray-plugin" {
		opts := v2rayObfsOption{Host: "bing.com"}
		if err := decoder.Decode(option.PluginOpts, &opts); err != nil {
			return nil, fmt.Errorf("ss %s initialize v2ray-plugin error: %s", server, err.Error())
		}
		obfsMode = opts.Mode
		var tlsConfig *tls.Config
		if opts.TLS {
			tlsConfig = &tls.Config{
				ServerName:         opts.Host,
				InsecureSkipVerify: opts.SkipCertVerify,
				ClientSessionCache: getClientSessionCache(),
			}
		}
		wsOption = &v2rayObfs.WebsocketOption{
			Host:      opts.Host,
			Path:      opts.Path,
			Headers:   opts.Headers,
			TLSConfig: tlsConfig,
		}
	}

	return &ShadowSocks{
		Base: &Base{
			name: option.Name,
			tp:   C.Shadowsocks,
		},
		server: server,
		cipher: ciph,

		obfsMode:   obfsMode,
		wsOption:   wsOption,
		obfsOption: obfsOption,
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
