package outboundgroup

import (
	"context"
	"encoding/json"
	"net"

	"github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/common/murmur3"
	"github.com/Dreamacro/clash/common/singledo"
	C "github.com/Dreamacro/clash/constant"

	"golang.org/x/net/publicsuffix"
)

type LoadBalance struct {
	*outbound.Base
	single    *singledo.Single
	maxRetry  int
	providers []provider.ProxyProvider
}

func getKey(metadata *C.Metadata) string {
	if metadata.Host != "" {
		// ip host
		if ip := net.ParseIP(metadata.Host); ip != nil {
			return metadata.Host
		}

		if etld, err := publicsuffix.EffectiveTLDPlusOne(metadata.Host); err == nil {
			return etld
		}
	}

	if metadata.DstIP == nil {
		return ""
	}

	return metadata.DstIP.String()
}

func jumpHash(key uint64, buckets int32) int32 {
	var b, j int64

	for j < int64(buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(int64(1)<<31) / float64((key>>33)+1)))
	}

	return int32(b)
}

func (lb *LoadBalance) DialContext(ctx context.Context, metadata *C.Metadata) (c C.Conn, err error) {
	defer func() {
		if err == nil {
			c.AppendToChains(lb)
		}
	}()

	proxy := lb.Unwrap(metadata)

	c, err = proxy.DialContext(ctx, metadata)
	return
}

func (lb *LoadBalance) DialUDP(metadata *C.Metadata) (pc C.PacketConn, err error) {
	defer func() {
		if err == nil {
			pc.AppendToChains(lb)
		}
	}()

	proxy := lb.Unwrap(metadata)

	return proxy.DialUDP(metadata)
}

func (lb *LoadBalance) SupportUDP() bool {
	return true
}

func (lb *LoadBalance) Unwrap(metadata *C.Metadata) C.Proxy {
	key := uint64(murmur3.Sum32([]byte(getKey(metadata))))
	proxies := lb.proxies()
	buckets := int32(len(proxies))
	for i := 0; i < lb.maxRetry; i, key = i+1, key+1 {
		idx := jumpHash(key, buckets)
		proxy := proxies[idx]
		if proxy.Alive() {
			return proxy
		}
	}

	return proxies[0]
}

func (lb *LoadBalance) proxies() []C.Proxy {
	elm, _, _ := lb.single.Do(func() (interface{}, error) {
		return getProvidersProxies(lb.providers), nil
	})

	return elm.([]C.Proxy)
}

func (lb *LoadBalance) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range lb.proxies() {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": lb.Type().String(),
		"all":  all,
	})
}

func NewLoadBalance(name string, providers []provider.ProxyProvider) *LoadBalance {
	return &LoadBalance{
		Base:      outbound.NewBase(name, "", C.LoadBalance, false),
		single:    singledo.NewSingle(defaultGetProxiesDuration),
		maxRetry:  3,
		providers: providers,
	}
}
