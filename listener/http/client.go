package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/Dreamacro/clash/adapter/inbound"
	C "github.com/Dreamacro/clash/constant"
)

const (
	defaultClientTimeout = time.Minute
	remoteAddrKey        = "remote-addr"
)

func newClient(in chan<- C.ConnContext) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, address string) (net.Conn, error) {
				remoteAddr := context.Value(remoteAddrKey)
				if remoteAddr == nil {
					return nil, errors.New("unknown remote addr")
				}

				if network != "tcp" && network != "tcp4" && network != "tcp6" {
					return nil, errors.New("unsupported network " + network)
				}

				left, right := net.Pipe()

				in <- inbound.NewHTTP(address, remoteAddr.(net.Addr), right)

				return left, nil
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: defaultClientTimeout,
	}
}
