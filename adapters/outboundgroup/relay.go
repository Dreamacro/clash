package outboundgroup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/common/singledo"
	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
)

type Relay struct {
	*outbound.Base
	single    *singledo.Single
	providers []provider.ProxyProvider
}

func (r *Relay) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	proxies := r.proxies(metadata)
	if len(proxies) == 0 {
		return nil, errors.New("Proxy does not exist")
	}
	first := proxies[0]
	last := proxies[len(proxies)-1]

	c, err := dialer.DialContext(ctx, "tcp", first.Addr())
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", first.Addr(), err)
	}
	tcpKeepAlive(c)

	var currentMeta *C.Metadata
	for _, proxy := range proxies[1:] {
		currentMeta, err = addrToMetadata(proxy.Addr())
		if err != nil {
			return nil, err
		}

		c, err = first.StreamConn(c, currentMeta)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", first.Addr(), err)
		}

		first = proxy
	}

	c, err = last.StreamConn(c, metadata)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", last.Addr(), err)
	}

	return outbound.NewConn(c, r), nil
}

func (r *Relay) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range r.rawProxies() {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": r.Type().String(),
		"all":  all,
	})
}

func (r *Relay) rawProxies() []C.Proxy {
	elm, _, _ := r.single.Do(func() (interface{}, error) {
		return getProvidersProxies(r.providers), nil
	})

	return elm.([]C.Proxy)
}

func (r *Relay) proxies(metadata *C.Metadata) []C.Proxy {
	proxies := r.rawProxies()

	for n, proxy := range proxies {
		subproxy := proxy.Unwrap(metadata)
		for subproxy != nil {
			proxies[n] = subproxy
			subproxy = subproxy.Unwrap(metadata)
		}
	}

	return proxies
}

func NewRelay(name string, providers []provider.ProxyProvider) *Relay {
	return &Relay{
		Base:      outbound.NewBase(name, "", C.Relay, false),
		single:    singledo.NewSingle(defaultGetProxiesDuration),
		providers: providers,
	}
}
