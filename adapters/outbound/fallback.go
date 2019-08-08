package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
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

func (f *Fallback) Dial(metadata *C.Metadata) (C.Conn, error) {
	proxy := f.findAliveProxy()
	c, err := proxy.Dial(metadata)
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
	go f.validTest()
Loop:
	for {
		select {
		case <-tick.C:
			go f.validTest()
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

func (f *Fallback) validTest() {
	wg := sync.WaitGroup{}
	wg.Add(len(f.proxies))

	for _, p := range f.proxies {
		go func(p C.Proxy) {
			p.URLTest(context.Background(), f.rawURL)
			wg.Done()
		}(p)
	}

	wg.Wait()
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
