package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/common/structure"
	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
	obfs "github.com/Dreamacro/clash/transport/simple-obfs"
	"github.com/Dreamacro/clash/transport/snell"
)

type Snell struct {
	*Base
	psk        []byte
	pool       *snell.Pool
	obfsOption *simpleObfsOption
	version    int
}

type SnellOption struct {
	Name     string                 `proxy:"name"`
	Server   string                 `proxy:"server"`
	Port     int                    `proxy:"port"`
	Psk      string                 `proxy:"psk"`
	Version  int                    `proxy:"version,omitempty"`
	ObfsOpts map[string]interface{} `proxy:"obfs-opts,omitempty"`
}

type streamOption struct {
	psk        []byte
	version    int
	addr       string
	obfsOption *simpleObfsOption
}

func streamConn(c net.Conn, option streamOption) *snell.Snell {
	switch option.obfsOption.Mode {
	case "tls":
		c = obfs.NewTLSObfs(c, option.obfsOption.Host)
	case "http":
		_, port, _ := net.SplitHostPort(option.addr)
		c = obfs.NewHTTPObfs(c, option.obfsOption.Host, port)
	}
	return snell.StreamConn(c, option.psk, option.version)
}

// StreamConn implements C.ProxyAdapter
func (s *Snell) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	c = streamConn(c, streamOption{s.psk, s.version, s.addr, s.obfsOption})
	port, _ := strconv.Atoi(metadata.DstPort)
	err := snell.WriteHeader(c, metadata.String(), uint(port), s.version)
	return c, err
}

// DialContext implements C.ProxyAdapter
func (s *Snell) DialContext(ctx context.Context, metadata *C.Metadata) (_ C.Conn, err error) {
	if s.version == snell.Version2 {
		c, err := s.pool.Get()
		if err != nil {
			return nil, err
		}

		port, _ := strconv.Atoi(metadata.DstPort)
		if err = snell.WriteHeader(c, metadata.String(), uint(port), s.version); err != nil {
			c.Close()
			return nil, err
		}
		return NewConn(c, s), err
	}

	c, err := dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", s.addr, err)
	}
	tcpKeepAlive(c)

	defer safeConnClose(c, err)

	c, err = s.StreamConn(c, metadata)
	return NewConn(c, s), err
}

func NewSnell(option SnellOption) (*Snell, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	psk := []byte(option.Psk)

	decoder := structure.NewDecoder(structure.Option{TagName: "obfs", WeaklyTypedInput: true})
	obfsOption := &simpleObfsOption{Host: "bing.com"}
	if err := decoder.Decode(option.ObfsOpts, obfsOption); err != nil {
		return nil, fmt.Errorf("snell %s initialize obfs error: %w", addr, err)
	}

	switch obfsOption.Mode {
	case "tls", "http", "":
		break
	default:
		return nil, fmt.Errorf("snell %s obfs mode error: %s", addr, obfsOption.Mode)
	}

	// backward compatible
	if option.Version == 0 {
		option.Version = snell.DefaultSnellVersion
	}
	if option.Version != snell.Version1 && option.Version != snell.Version2 {
		return nil, fmt.Errorf("snell version error: %d", option.Version)
	}

	s := &Snell{
		Base: &Base{
			name: option.Name,
			addr: addr,
			tp:   C.Snell,
		},
		psk:        psk,
		obfsOption: obfsOption,
		version:    option.Version,
	}

	if option.Version == snell.Version2 {
		s.pool = snell.NewPool(func(ctx context.Context) (*snell.Snell, error) {
			c, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return nil, err
			}

			tcpKeepAlive(c)
			return streamConn(c, streamOption{psk, option.Version, addr, obfsOption}), nil
		})
	}
	return s, nil
}
