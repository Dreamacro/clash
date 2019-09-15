package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/Dreamacro/clash/common/picker"
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

func (u *URLTest) Dial(metadata *C.Metadata) (c C.Conn, err error) {
	for i := 0; i < 3; i++ {
		c, err = u.fast.Dial(metadata)
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

func (u *URLTest) loop() {
	tick := time.NewTicker(u.interval)
	go u.speedTest()
Loop:
	for {
		select {
		case <-tick.C:
			go u.speedTest()
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

func (u *URLTest) speedTest() {
	if !atomic.CompareAndSwapInt32(&u.once, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&u.once, 0)

	picker, ctx, cancel := picker.WithTimeout(context.Background(), defaultURLTestTimeout)
	defer cancel()
	for _, p := range u.proxies {
		proxy := p
		picker.Go(func() (interface{}, error) {
			_, err := proxy.URLTest(ctx, u.rawURL)
			if err != nil {
				return nil, err
			}
			return proxy, nil
		})
	}

	fast := picker.Wait()
	if fast != nil {
		u.fast = fast.(C.Proxy)
	}
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
