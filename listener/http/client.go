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
)

func newClient(source net.Addr, in chan<- C.ConnContext) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, address string) (net.Conn, error) {
				if network != "tcp" && network != "tcp4" && network != "tcp6" {
					return nil, errors.New("unsupported network " + network)
				}

				left, right := net.Pipe()

				in <- inbound.NewHTTP(address, source, right)

				return left, nil
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: defaultClientTimeout,
	}
}
