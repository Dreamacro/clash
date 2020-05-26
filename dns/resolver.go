package dns

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/common/picker"
	"github.com/Dreamacro/clash/component/fakeip"
	"github.com/Dreamacro/clash/component/resolver"

	D "github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

var (
	globalSessionCache = tls.NewLRUClientSessionCache(64)
)

type dnsClient interface {
	Exchange(m *D.Msg) (msg *D.Msg, err error)
	ExchangeContext(ctx context.Context, m *D.Msg) (msg *D.Msg, err error)
}

type result struct {
	Msg   *D.Msg
	Error error
}

type Resolver struct {
	ipv6            bool
	mapping         bool
	fakeip          bool
	pool            *fakeip.Pool
	main            []dnsClient
	fallback        []dnsClient
	fallbackFilters []fallbackFilter
	group           singleflight.Group
	lruCache        *cache.LruCache
}

// ResolveIP request with TypeA and TypeAAAA, priority return TypeA
func (r *Resolver) ResolveIP(host string) (ip net.IP, err error) {
	ch := make(chan net.IP, 1)
	go func() {
		defer close(ch)
		ip, err := r.resolveIP(host, D.TypeAAAA)
		if err != nil {
			return
		}
		ch <- ip
	}()

	ip, err = r.resolveIP(host, D.TypeA)
	if err == nil {
		return
	}

	ip, open := <-ch
	if !open {
		return nil, resolver.ErrIPNotFound
	}

	return ip, nil
}

// ResolveIPv4 request with TypeA
func (r *Resolver) ResolveIPv4(host string) (ip net.IP, err error) {
	return r.resolveIP(host, D.TypeA)
}

// ResolveIPv6 request with TypeAAAA
func (r *Resolver) ResolveIPv6(host string) (ip net.IP, err error) {
	return r.resolveIP(host, D.TypeAAAA)
}

func (r *Resolver) shouldFallback(ip net.IP) bool {
	for _, filter := range r.fallbackFilters {
		if filter.Match(ip) {
			return true
		}
	}
	return false
}

// Exchange a batch of dns request, and it use cache
func (r *Resolver) Exchange(m *D.Msg) (msg *D.Msg, err error) {
	if len(m.Question) == 0 {
		return nil, errors.New("should have one question at least")
	}

	q := m.Question[0]
	cache, expireTime, hit := r.lruCache.GetWithExpire(q.String())
	if hit {
		now := time.Now()
		msg = cache.(*D.Msg).Copy()
		if expireTime.Before(now) {
			setMsgTTL(msg, uint32(1)) // Continue fetch
			go r.exchangeWithoutCache(m)
		} else {
			setMsgTTL(msg, uint32(expireTime.Sub(time.Now()).Seconds()))
		}
		return
	}
	return r.exchangeWithoutCache(m)
}

// ExchangeWithoutCache a batch of dns request, and it do NOT GET from cache
func (r *Resolver) exchangeWithoutCache(m *D.Msg) (msg *D.Msg, err error) {
	q := m.Question[0]

	defer func() {
		if msg == nil {
			return
		}

		putMsgToCache(r.lruCache, q.String(), msg)
		if r.mapping {
			ips := r.msgToIP(msg)
			for _, ip := range ips {
				putMsgToCache(r.lruCache, ip.String(), msg)
			}
		}
	}()

	ret, err, shared := r.group.Do(q.String(), func() (interface{}, error) {
		isIPReq := isIPRequest(q)
		if isIPReq {
			return r.fallbackExchange(m)
		}

		return r.batchExchange(r.main, m)
	})

	if err == nil {
		msg = ret.(*D.Msg)
		if shared {
			msg = msg.Copy()
		}
	}

	return
}

// IPToHost return fake-ip or redir-host mapping host
func (r *Resolver) IPToHost(ip net.IP) (string, bool) {
	if r.fakeip {
		return r.pool.LookBack(ip)
	}

	cache, _ := r.lruCache.Get(ip.String())
	if cache == nil {
		return "", false
	}
	fqdn := cache.(*D.Msg).Question[0].Name
	return strings.TrimRight(fqdn, "."), true
}

func (r *Resolver) IsMapping() bool {
	return r.mapping
}

// FakeIPEnabled returns if fake-ip is enabled
func (r *Resolver) FakeIPEnabled() bool {
	return r.fakeip
}

// IsFakeIP determine if given ip is a fake-ip
func (r *Resolver) IsFakeIP(ip net.IP) bool {
	if r.FakeIPEnabled() {
		return r.pool.Exist(ip)
	}
	return false
}

func (r *Resolver) batchExchange(clients []dnsClient, m *D.Msg) (msg *D.Msg, err error) {
	fast, ctx := picker.WithTimeout(context.Background(), time.Second*5)
	for _, client := range clients {
		r := client
		fast.Go(func() (interface{}, error) {
			m, err := r.ExchangeContext(ctx, m)
			if err != nil {
				return nil, err
			} else if m.Rcode == D.RcodeServerFailure || m.Rcode == D.RcodeRefused {
				return nil, errors.New("server failure")
			}
			return m, nil
		})
	}

	elm := fast.Wait()
	if elm == nil {
		err := errors.New("All DNS requests failed")
		if fErr := fast.Error(); fErr != nil {
			err = fmt.Errorf("%w, first error: %s", err, fErr.Error())
		}
		return nil, err
	}

	msg = elm.(*D.Msg)
	return
}

func (r *Resolver) fallbackExchange(m *D.Msg) (msg *D.Msg, err error) {
	msgCh := r.asyncExchange(r.main, m)
	if r.fallback == nil {
		res := <-msgCh
		msg, err = res.Msg, res.Error
		return
	}
	fallbackMsg := r.asyncExchange(r.fallback, m)
	res := <-msgCh
	if res.Error == nil {
		if ips := r.msgToIP(res.Msg); len(ips) != 0 {
			if !r.shouldFallback(ips[0]) {
				msg = res.Msg
				err = res.Error
				return msg, err
			}
		}
	}

	res = <-fallbackMsg
	msg, err = res.Msg, res.Error
	return
}

func (r *Resolver) resolveIP(host string, dnsType uint16) (ip net.IP, err error) {
	ip = net.ParseIP(host)
	if ip != nil {
		isIPv4 := ip.To4() != nil
		if dnsType == D.TypeAAAA && !isIPv4 {
			return ip, nil
		} else if dnsType == D.TypeA && isIPv4 {
			return ip, nil
		} else {
			return nil, resolver.ErrIPVersion
		}
	}

	query := &D.Msg{}
	query.SetQuestion(D.Fqdn(host), dnsType)

	msg, err := r.Exchange(query)
	if err != nil {
		return nil, err
	}

	ips := r.msgToIP(msg)
	ipLength := len(ips)
	if ipLength == 0 {
		return nil, resolver.ErrIPNotFound
	}

	ip = ips[rand.Intn(ipLength)]
	return
}

func (r *Resolver) msgToIP(msg *D.Msg) []net.IP {
	ips := []net.IP{}

	for _, answer := range msg.Answer {
		switch ans := answer.(type) {
		case *D.AAAA:
			ips = append(ips, ans.AAAA)
		case *D.A:
			ips = append(ips, ans.A)
		}
	}

	return ips
}

func (r *Resolver) asyncExchange(client []dnsClient, msg *D.Msg) <-chan *result {
	ch := make(chan *result, 1)
	go func() {
		res, err := r.batchExchange(client, msg)
		ch <- &result{Msg: res, Error: err}
	}()
	return ch
}

type NameServer struct {
	Net  string
	Addr string
}

type FallbackFilter struct {
	GeoIP  bool
	IPCIDR []*net.IPNet
}

type Config struct {
	Main, Fallback []NameServer
	Default        []NameServer
	IPv6           bool
	EnhancedMode   EnhancedMode
	FallbackFilter FallbackFilter
	Pool           *fakeip.Pool
}

func New(config Config) *Resolver {
	defaultResolver := &Resolver{
		main:     transform(config.Default, nil),
		lruCache: cache.NewLRUCache(cache.WithSize(4096), cache.WithStale(true)),
	}

	r := &Resolver{
		ipv6:     config.IPv6,
		main:     transform(config.Main, defaultResolver),
		lruCache: cache.NewLRUCache(cache.WithSize(4096), cache.WithStale(true)),
		mapping:  config.EnhancedMode == MAPPING,
		fakeip:   config.EnhancedMode == FAKEIP,
		pool:     config.Pool,
	}

	if len(config.Fallback) != 0 {
		r.fallback = transform(config.Fallback, defaultResolver)
	}

	fallbackFilters := []fallbackFilter{}
	if config.FallbackFilter.GeoIP {
		fallbackFilters = append(fallbackFilters, &geoipFilter{})
	}
	for _, ipnet := range config.FallbackFilter.IPCIDR {
		fallbackFilters = append(fallbackFilters, &ipnetFilter{ipnet: ipnet})
	}
	r.fallbackFilters = fallbackFilters

	return r
}
