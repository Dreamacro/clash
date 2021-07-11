package outboundgroup

import (
	"context"
	"encoding/json"
	"net"

	"github.com/Dreamacro/clash/adapter/outbound"
	"github.com/Dreamacro/clash/common/singledo"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/constant/provider"
)

type Relay struct {
	*outbound.Base
	single    *singledo.Single
	providers []provider.ProxyProvider
}

// DialContext implements C.ProxyAdapter
func (r *Relay) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var dialContext outbound.DialContextFunc
	for _, proxy := range r.proxies(true) {
		proxy := proxy
		dial := dialContext

		dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			ctx = outbound.WithDialContext(ctx, dial)
			return proxy.DialContext(ctx, network, address)
		}
	}

	ctx = outbound.WithDialContext(ctx, dialContext)
	return outbound.DialContextDecorated(ctx, network, address, func(conn net.Conn) (net.Conn, error) {
		return outbound.WithRouteHop(conn, r), nil
	})
}

// MarshalJSON implements C.ProxyAdapter
func (r *Relay) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range r.proxies(false) {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": r.Type().String(),
		"all":  all,
	})
}

func (r *Relay) proxies(touch bool) []C.Proxy {
	elm, _, _ := r.single.Do(func() (interface{}, error) {
		return getProvidersProxies(r.providers, touch), nil
	})

	return elm.([]C.Proxy)
}

func NewRelay(options *GroupCommonOption, providers []provider.ProxyProvider) *Relay {
	return &Relay{
		Base:      outbound.NewBase(options.Name, "", C.Relay, false),
		single:    singledo.NewSingle(defaultGetProxiesDuration),
		providers: providers,
	}
}
