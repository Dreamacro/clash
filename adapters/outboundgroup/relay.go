package outboundgroup

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/common/singledo"
	C "github.com/Dreamacro/clash/constant"
)

type Relay struct {
	*outbound.Base
	single    *singledo.Single
	providers []provider.ProxyProvider
}

func (r *Relay) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	proxies := r.rawProxies()
	if len(proxies) == 0 {
		return nil, errors.New("Proxy does not exist")
	}
	dialFunc := proxies[0].DialContext
	for _, proxy := range proxies[1:] {
		previousDialFunc := dialFunc
		dialFunc = func(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
			metadata.DialContext = func(ctx context.Context, network, address string) (conn net.Conn, err error) {
				curMetaData, err := addrToMetadata(address)
				if err != nil {
					return nil, err
				}
				return previousDialFunc(ctx, curMetaData)
			}
			return proxy.DialContext(ctx, metadata)
		}
	}

	return dialFunc(ctx, metadata)
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
