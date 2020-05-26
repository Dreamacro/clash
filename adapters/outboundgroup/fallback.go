package outboundgroup

import (
	"context"
	"encoding/json"

	"github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/common/singledo"
	C "github.com/Dreamacro/clash/constant"
)

type Fallback struct {
	*outbound.Base
	single    *singledo.Single
	providers []provider.ProxyProvider
}

func (f *Fallback) Now() string {
	proxy := f.findAliveProxy()
	return proxy.Name()
}

func (f *Fallback) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	proxy := f.findAliveProxy()
	c, err := proxy.DialContext(ctx, metadata)
	if err == nil {
		c.AppendToChains(f)
	}
	return c, err
}

func (f *Fallback) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	proxy := f.findAliveProxy()
	pc, err := proxy.DialUDP(metadata)
	if err == nil {
		pc.AppendToChains(f)
	}
	return pc, err
}

func (f *Fallback) SupportUDP() bool {
	proxy := f.findAliveProxy()
	return proxy.SupportUDP()
}

func (f *Fallback) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range f.proxies() {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": f.Type().String(),
		"now":  f.Now(),
		"all":  all,
	})
}

func (f *Fallback) Unwrap(metadata *C.Metadata) C.Proxy {
	proxy := f.findAliveProxy()
	return proxy
}

func (f *Fallback) proxies() []C.Proxy {
	elm, _, _ := f.single.Do(func() (interface{}, error) {
		return getProvidersProxies(f.providers), nil
	})

	return elm.([]C.Proxy)
}

func (f *Fallback) findAliveProxy() C.Proxy {
	proxies := f.proxies()
	for _, proxy := range proxies {
		if proxy.Alive() {
			return proxy
		}
	}

	return f.proxies()[0]
}

func NewFallback(name string, providers []provider.ProxyProvider) *Fallback {
	return &Fallback{
		Base:      outbound.NewBase(name, "", C.Fallback, false),
		single:    singledo.NewSingle(defaultGetProxiesDuration),
		providers: providers,
	}
}
