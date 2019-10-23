package adapters

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/common/structure"
	obfs "github.com/Dreamacro/clash/component/simple-obfs"
	"github.com/Dreamacro/clash/component/snell"
	C "github.com/Dreamacro/clash/constant"
)

type Snell struct {
	*Base
	server     string
	psk        []byte
	obfsOption *simpleObfsOption
}

type SnellOption struct {
	Name     string                 `proxy:"name"`
	Server   string                 `proxy:"server"`
	Port     int                    `proxy:"port"`
	Psk      string                 `proxy:"psk"`
	ObfsOpts map[string]interface{} `proxy:"obfs-opts,omitempty"`
}

func (s *Snell) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialContext(ctx, "tcp", s.server)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %s", s.server, err.Error())
	}
	tcpKeepAlive(c)
	switch s.obfsOption.Mode {
	case "tls":
		c = obfs.NewTLSObfs(c, s.obfsOption.Host)
	case "http":
		_, port, _ := net.SplitHostPort(s.server)
		c = obfs.NewHTTPObfs(c, s.obfsOption.Host, port)
	}
	c = snell.StreamConn(c, s.psk)
	port, _ := strconv.Atoi(metadata.DstPort)
	err = snell.WriteHeader(c, metadata.String(), uint(port))
	return newConn(c, s), err
}

func (s *Snell) HealthCheck(ctx context.Context, url string) (uint16, error) {
	return urlTest(ctx, s, url)
}

func NewSnell(option SnellOption) (*Snell, error) {
	server := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	psk := []byte(option.Psk)

	decoder := structure.NewDecoder(structure.Option{TagName: "obfs", WeaklyTypedInput: true})
	obfsOption := &simpleObfsOption{Host: "bing.com"}
	if err := decoder.Decode(option.ObfsOpts, obfsOption); err != nil {
		return nil, fmt.Errorf("snell %s initialize obfs error: %s", server, err.Error())
	}

	if obfsOption.Mode != "tls" && obfsOption.Mode != "http" {
		return nil, fmt.Errorf("snell %s obfs mode error: %s", server, obfsOption.Mode)
	}

	return &Snell{
		Base: &Base{
			name: option.Name,
			tp:   C.Snell,
		},
		server:     server,
		psk:        psk,
		obfsOption: obfsOption,
	}, nil
}
