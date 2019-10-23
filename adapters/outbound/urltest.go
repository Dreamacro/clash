package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"time"

	C "github.com/Dreamacro/clash/constant"
)

type URLTest struct {
	*Base
	proxies  []C.Proxy
	rawURL   string
	fast     C.Proxy
	interval time.Duration
	done     chan struct{}
	once     int32
}

type URLTestOption struct {
	Name     string   `proxy:"name"`
	Proxies  []string `proxy:"proxies"`
	URL      string   `proxy:"url"`
	Interval int      `proxy:"interval"`
}

func (u *URLTest) Now() string {
	return u.fast.Name()
}

func (u *URLTest) DialContext(ctx context.Context, metadata *C.Metadata) (c C.Conn, err error) {
	for i := 0; i < 3; i++ {
		c, err = u.fast.DialContext(ctx, metadata)
		if err == nil {
			c.AppendToChains(u)
			return
		}
		u.fallback()
	}
	return
}

func (u *URLTest) DialUDP(metadata *C.Metadata) (C.PacketConn, net.Addr, error) {
	pc, addr, err := u.fast.DialUDP(metadata)
	if err == nil {
		pc.AppendToChains(u)
	}
	return pc, addr, err
}

func (u *URLTest) SupportUDP() bool {
	return u.fast.SupportUDP()
}

func (u *URLTest) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range u.proxies {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": u.Type().String(),
		"now":  u.Now(),
		"all":  all,
	})
}

func (u *URLTest) Destroy() {
	u.done <- struct{}{}
}

func (u *URLTest) HealthCheck(ctx context.Context, url string) (uint16, error) {
	if url == "" {
		url = u.rawURL
	}
	fast, err := u.healthCheck(ctx, url, false)
	if err != nil {
		return 0, err
	}
	return fast.LastDelay(), nil
}

func (u *URLTest) healthCheck(ctx context.Context, url string, checkAllInGroup bool) (C.Proxy, error) {
	if !atomic.CompareAndSwapInt32(&u.once, 0, 1) {
		return nil, errAgain
	}
	defer atomic.StoreInt32(&u.once, 0)
	checkSingle := func(ctx context.Context, proxy C.Proxy) (interface{}, error) {
		_, err := proxy.HealthCheck(ctx, url)
		if err != nil {
			return nil, err
		}
		return proxy, nil
	}
	result, err := groupHealthCheck(ctx, u.proxies, url, checkAllInGroup, checkSingle)
	if err == nil {
		fast, _ := result.(C.Proxy)
		return fast, nil
	}
	return nil, err
}

func (u *URLTest) loop() {
	tick := time.NewTicker(u.interval)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go u.healthCheck(ctx, u.rawURL, false)
Loop:
	for {
		select {
		case <-tick.C:
			go u.healthCheck(ctx, u.rawURL, false)
		case <-u.done:
			break Loop
		}
	}
}

func (u *URLTest) fallback() {
	fast := u.proxies[0]
	min := fast.LastDelay()
	for _, proxy := range u.proxies[1:] {
		if !proxy.Alive() {
			continue
		}

		delay := proxy.LastDelay()
		if delay < min {
			fast = proxy
			min = delay
		}
	}
	u.fast = fast
}

func NewURLTest(option URLTestOption, proxies []C.Proxy) (*URLTest, error) {
	_, err := urlToMetadata(option.URL)
	if err != nil {
		return nil, err
	}
	if len(proxies) < 1 {
		return nil, errors.New("The number of proxies cannot be 0")
	}

	interval := time.Duration(option.Interval) * time.Second
	urlTest := &URLTest{
		Base: &Base{
			name: option.Name,
			tp:   C.URLTest,
		},
		proxies:  proxies[:],
		rawURL:   option.URL,
		fast:     proxies[0],
		interval: interval,
		done:     make(chan struct{}),
		once:     0,
	}
	go urlTest.loop()
	return urlTest, nil
}
