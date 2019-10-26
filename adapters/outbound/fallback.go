package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

type Fallback struct {
	*Base
	proxies  []C.Proxy
	rawURL   string
	interval time.Duration
	done     chan struct{}
}

type FallbackOption struct {
	Name     string   `proxy:"name"`
	Proxies  []string `proxy:"proxies"`
	URL      string   `proxy:"url"`
	Interval int      `proxy:"interval"`
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

func (f *Fallback) DialUDP(metadata *C.Metadata) (C.PacketConn, net.Addr, error) {
	proxy := f.findAliveProxy()
	pc, addr, err := proxy.DialUDP(metadata)
	if err == nil {
		pc.AppendToChains(f)
	}
	return pc, addr, err
}

func (f *Fallback) SupportUDP() bool {
	proxy := f.findAliveProxy()
	return proxy.SupportUDP()
}

func (f *Fallback) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range f.proxies {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": f.Type().String(),
		"now":  f.Now(),
		"all":  all,
	})
}

func (f *Fallback) Destroy() {
	f.done <- struct{}{}
}

func (f *Fallback) loop() {
	tick := time.NewTicker(f.interval)
	back := WithGroupKey(context.Background(), MakeGroupKey(f.Name(), f.rawURL, defaultURLTestTimeout.Nanoseconds()))
	go func() {
		ctx, cancel := context.WithTimeout(back, defaultURLTestTimeout)
		defer cancel()
		f.healthCheck(ctx, f.rawURL, true)
	}()
Loop:
	for {
		select {
		case <-tick.C:
			go func() {
				ctx, cancel := context.WithTimeout(back, defaultURLTestTimeout)
				defer cancel()
				f.healthCheck(ctx, f.rawURL, true)
			}()
		case <-f.done:
			break Loop
		}
	}
}

func (f *Fallback) findAliveProxy() C.Proxy {
	for _, proxy := range f.proxies {
		if proxy.Alive() {
			return proxy
		}
	}
	return f.proxies[0]
}

func (f *Fallback) HealthCheck(ctx context.Context, url string) (delay uint16, err error) {
	if url == "" {
		url = f.rawURL
	}
	return f.healthCheck(ctx, url, false)
}

func (f *Fallback) healthCheck(ctx context.Context, url string, checkAllInGroup bool) (delay uint16, err error) {
	checkSingle := func(ctx context.Context, proxy C.Proxy) (interface{}, error) {
		_, err := proxy.HealthCheck(ctx, url)
		if err != nil {
			return nil, err
		}
		return proxy, nil
	}
	select {
	case <-ctx.Done():
		return 0, errTimeout
	case result := <-healthCheckGroup.DoChan(getGroupKey(ctx), func() (interface{}, error) {
		return groupHealthCheck(ctx, f.proxies, url, checkAllInGroup, checkSingle)
	}):
		if result.Err == nil {
			fast := result.Val.(C.Proxy)
			return fast.LastDelay(), nil
		}
		return 0, result.Err
	}
}

func NewFallback(option FallbackOption, proxies []C.Proxy) (*Fallback, error) {
	_, err := urlToMetadata(option.URL)
	if err != nil {
		return nil, err
	}

	if len(proxies) < 1 {
		return nil, errors.New("The number of proxies cannot be 0")
	}

	interval := time.Duration(option.Interval) * time.Second

	Fallback := &Fallback{
		Base: &Base{
			name: option.Name,
			tp:   C.Fallback,
		},
		proxies:  proxies,
		rawURL:   option.URL,
		interval: interval,
		done:     make(chan struct{}),
	}
	go Fallback.loop()
	return Fallback, nil
}
