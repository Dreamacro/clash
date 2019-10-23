package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/Dreamacro/clash/common/murmur3"
	C "github.com/Dreamacro/clash/constant"

	"golang.org/x/net/publicsuffix"
)

type LoadBalance struct {
	*Base
	proxies  []C.Proxy
	maxRetry int
	rawURL   string
	interval time.Duration
	done     chan struct{}
	once     int32
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

	key := uint64(murmur3.Sum32([]byte(getKey(metadata))))
	buckets := int32(len(lb.proxies))
	for i := 0; i < lb.maxRetry; i, key = i+1, key+1 {
		idx := jumpHash(key, buckets)
		proxy := lb.proxies[idx]
		if proxy.Alive() {
			c, err = proxy.DialContext(ctx, metadata)
			return
		}
	}
	c, err = lb.proxies[0].DialContext(ctx, metadata)
	return
}

func (lb *LoadBalance) DialUDP(metadata *C.Metadata) (pc C.PacketConn, addr net.Addr, err error) {
	defer func() {
		if err == nil {
			pc.AppendToChains(lb)
		}
	}()

	key := uint64(murmur3.Sum32([]byte(getKey(metadata))))
	buckets := int32(len(lb.proxies))
	for i := 0; i < lb.maxRetry; i, key = i+1, key+1 {
		idx := jumpHash(key, buckets)
		proxy := lb.proxies[idx]
		if proxy.Alive() {
			return proxy.DialUDP(metadata)
		}
	}

	return lb.proxies[0].DialUDP(metadata)
}

func (lb *LoadBalance) SupportUDP() bool {
	return true
}

func (lb *LoadBalance) Destroy() {
	lb.done <- struct{}{}
}

func (lb *LoadBalance) HealthCheck(ctx context.Context, url string) (uint16, error) {
	if url == "" {
		url = lb.rawURL
	}
	return lb.healthCheck(ctx, url, false)
}

func (lb *LoadBalance) healthCheck(ctx context.Context, url string, checkAllInGroup bool) (uint16, error) {
	if !atomic.CompareAndSwapInt32(&lb.once, 0, 1) {
		return 0, errAgain
	}
	defer atomic.StoreInt32(&lb.once, 0)
	checkSingle := func(ctx context.Context, proxy C.Proxy) (interface{}, error) {
		return proxy.HealthCheck(ctx, url)
	}

	result, err := groupHealthCheck(ctx, lb.proxies, url, checkAllInGroup, checkSingle)
	if err == nil {
		if delay, ok := result.(uint16); ok {
			return delay, nil
		}
	}
	return 0, err
}

func (lb *LoadBalance) loop() {
	tick := time.NewTicker(lb.interval)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go lb.healthCheck(ctx, lb.rawURL, true)
Loop:
	for {
		select {
		case <-tick.C:
			go lb.healthCheck(ctx, lb.rawURL, true)
		case <-lb.done:
			break Loop
		}
	}
}

func (lb *LoadBalance) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range lb.proxies {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": lb.Type().String(),
		"all":  all,
	})
}

type LoadBalanceOption struct {
	Name     string   `proxy:"name"`
	Proxies  []string `proxy:"proxies"`
	URL      string   `proxy:"url"`
	Interval int      `proxy:"interval"`
}

func NewLoadBalance(option LoadBalanceOption, proxies []C.Proxy) (*LoadBalance, error) {
	if len(proxies) == 0 {
		return nil, errors.New("Provide at least one proxy")
	}

	interval := time.Duration(option.Interval) * time.Second

	lb := &LoadBalance{
		Base: &Base{
			name: option.Name,
			tp:   C.LoadBalance,
		},
		proxies:  proxies,
		maxRetry: 3,
		rawURL:   option.URL,
		interval: interval,
		done:     make(chan struct{}),
	}
	go lb.loop()
	return lb, nil
}
