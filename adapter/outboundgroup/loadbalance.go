package outboundgroup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/Dreamacro/clash/adapter/outbound"
	"github.com/Dreamacro/clash/common/murmur3"
	"github.com/Dreamacro/clash/common/singledo"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/constant/provider"

	"golang.org/x/net/publicsuffix"
)

type strategyFn = func(proxies []C.Proxy, address string) C.Proxy

type LoadBalance struct {
	*outbound.Base
	disableUDP bool
	single     *singledo.Single
	providers  []provider.ProxyProvider
	strategyFn strategyFn
}

var errStrategy = errors.New("unsupported strategy")

func parseStrategy(config map[string]interface{}) string {
	if elm, ok := config["strategy"]; ok {
		if strategy, ok := elm.(string); ok {
			return strategy
		}
	}
	return "consistent-hashing"
}

func getKey(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}

	if net.ParseIP(host) != nil {
		return host
	}

	if etld, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return etld
	}

	return host
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

// DialContext implements C.ProxyAdapter
func (lb *LoadBalance) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	ctx = outbound.WithDialContext(ctx, lb.strategyFn(lb.proxies(true), address).DialContext)
	return outbound.DialContextDecorated(ctx, network, address, func(conn net.Conn) (net.Conn, error) {
		return outbound.WithRouteHop(conn, lb), nil
	})
}

// SupportUDP implements C.ProxyAdapter
func (lb *LoadBalance) SupportUDP() bool {
	return !lb.disableUDP
}

func strategyRoundRobin() strategyFn {
	idx := 0
	return func(proxies []C.Proxy, address string) C.Proxy {
		length := len(proxies)
		for i := 0; i < length; i++ {
			idx = (idx + 1) % length
			proxy := proxies[idx]
			if proxy.Alive() {
				return proxy
			}
		}

		return proxies[0]
	}
}

func strategyConsistentHashing() strategyFn {
	maxRetry := 5
	return func(proxies []C.Proxy, address string) C.Proxy {
		key := uint64(murmur3.Sum32([]byte(getKey(address))))
		buckets := int32(len(proxies))
		for i := 0; i < maxRetry; i, key = i+1, key+1 {
			idx := jumpHash(key, buckets)
			proxy := proxies[idx]
			if proxy.Alive() {
				return proxy
			}
		}

		return proxies[0]
	}
}

func (lb *LoadBalance) proxies(touch bool) []C.Proxy {
	elm, _, _ := lb.single.Do(func() (interface{}, error) {
		return getProvidersProxies(lb.providers, touch), nil
	})

	return elm.([]C.Proxy)
}

// MarshalJSON implements C.ProxyAdapter
func (lb *LoadBalance) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range lb.proxies(false) {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": lb.Type().String(),
		"all":  all,
	})
}

func NewLoadBalance(options *GroupCommonOption, providers []provider.ProxyProvider, strategy string) (lb *LoadBalance, err error) {
	var strategyFn strategyFn
	switch strategy {
	case "consistent-hashing":
		strategyFn = strategyConsistentHashing()
	case "round-robin":
		strategyFn = strategyRoundRobin()
	default:
		return nil, fmt.Errorf("%w: %s", errStrategy, strategy)
	}
	return &LoadBalance{
		Base:       outbound.NewBase(options.Name, "", C.LoadBalance, false),
		single:     singledo.NewSingle(defaultGetProxiesDuration),
		providers:  providers,
		strategyFn: strategyFn,
		disableUDP: options.DisableUDP,
	}, nil
}
