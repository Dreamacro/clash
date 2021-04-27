package outboundgroup

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/common/singledo"
	C "github.com/Dreamacro/clash/constant"
)

type urlTestOption func(*URLTest)

func urlTestWithTolerance(tolerance uint16) urlTestOption {
	return func(u *URLTest) {
		u.tolerance = tolerance
	}
}

type URLTest struct {
	*outbound.Base
	tolerance  uint16
	disableUDP bool
	fastNode   C.Proxy
	single     *singledo.Single
	fastSingle *singledo.Single
	providers  []provider.ProxyProvider
}

func (u *URLTest) Now() string {
	return u.fast(false).Name()
}

func (u *URLTest) DialContext(ctx context.Context, metadata *C.Metadata) (c C.Conn, err error) {
	c, err = u.fast(true).DialContext(ctx, metadata)
	if err == nil {
		c.AppendToChains(u)
	}
	return c, err
}

func (u *URLTest) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pc, err := u.fast(true).DialUDP(metadata)
	if err == nil {
		pc.AppendToChains(u)
	}
	return pc, err
}

func (u *URLTest) Unwrap(metadata *C.Metadata) C.Proxy {
	return u.fast(true)
}

func (u *URLTest) proxies(touch bool) []C.Proxy {
	elm, _, _ := u.single.Do(func() (interface{}, error) {
		return getProvidersProxies(u.providers, touch), nil
	})

	return elm.([]C.Proxy)
}

func (u *URLTest) fast(touch bool) C.Proxy {
	elm, _, _ := u.fastSingle.Do(func() (interface{}, error) {
		proxies := u.proxies(touch)
		fast := proxies[0]
		oldFast := u.fastNode
		u.fastNode = nil
		min := fast.LastDelay()
		if oldFast != nil {
			for _, proxy := range proxies[:] {
				if proxy.Name() == oldFast.Name() {
					u.fastNode = oldFast
					break
				}
			}
		}
		for _, proxy := range proxies[1:] {
			if !proxy.Alive() {
				continue
			}

			delay := proxy.LastDelay()
			if delay < min {
				fast = proxy
				min = delay
			}
		}

		// tolerance
		if u.fastNode == nil || !u.fastNode.Alive() || u.fastNode.LastDelay() > fast.LastDelay()+u.tolerance {
			u.fastNode = fast
		}

		return u.fastNode, nil
	})

	return elm.(C.Proxy)
}

func (u *URLTest) SupportUDP() bool {
	if u.disableUDP {
		return false
	}

	return u.fast(false).SupportUDP()
}

func (u *URLTest) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range u.proxies(false) {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": u.Type().String(),
		"now":  u.Now(),
		"all":  all,
	})
}

func parseURLTestOption(config map[string]interface{}) []urlTestOption {
	opts := []urlTestOption{}

	// tolerance
	if elm, ok := config["tolerance"]; ok {
		if tolerance, ok := elm.(int); ok {
			opts = append(opts, urlTestWithTolerance(uint16(tolerance)))
		}
	}

	return opts
}

func NewURLTest(commonOptions *GroupCommonOption, providers []provider.ProxyProvider, options ...urlTestOption) *URLTest {
	urlTest := &URLTest{
		Base:       outbound.NewBase(commonOptions.Name, "", "", C.URLTest, false, 0, 0, 0, 1),
		single:     singledo.NewSingle(defaultGetProxiesDuration),
		fastSingle: singledo.NewSingle(time.Second * 10),
		providers:  providers,
		disableUDP: commonOptions.DisableUDP,
	}

	for _, option := range options {
		option(urlTest)
	}

	return urlTest
}
