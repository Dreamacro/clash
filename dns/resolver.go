package dns

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/common/picker"
	trie "github.com/Dreamacro/clash/component/domain-trie"
	"github.com/Dreamacro/clash/component/fakeip"
	C "github.com/Dreamacro/clash/constant"

	D "github.com/miekg/dns"
	geoip2 "github.com/oschwald/geoip2-golang"
	"golang.org/x/sync/singleflight"
)

var (
	// DefaultResolver aim to resolve ip
	DefaultResolver *Resolver

	// DefaultHosts aim to resolve hosts
	DefaultHosts = trie.New()
)

var (
	globalSessionCache = tls.NewLRUClientSessionCache(64)

	mmdb *geoip2.Reader
	once sync.Once
)

type resolver interface {
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
	main            []resolver
	fallback        []resolver
	fallbackFilters []fallbackFilter
	group           singleflight.Group
	cache           *cache.Cache
}

// ResolveIP request with TypeA and TypeAAAA, priority return TypeAAAA
func (r *Resolver) ResolveIP(host string) (ip net.IP, err error) {
	ch := make(chan net.IP)
	go func() {
		defer close(ch)
		ip, err := r.resolveIP(host, D.TypeA)
		if err != nil {
			return
		}
		ch <- ip
	}()

	ip, err = r.resolveIP(host, D.TypeAAAA)
	if err == nil {
		go func() {
			<-ch
		}()
		return
	}

	ip, open := <-ch
	if !open {
		return nil, errIPNotFound
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
	cache, expireTime := r.cache.GetWithExpire(q.String())
	if cache != nil {
		msg = cache.(*D.Msg).Copy()
		setMsgTTL(msg, uint32(expireTime.Sub(time.Now()).Seconds()))
		return
	}
	defer func() {
		if msg == nil {
			return
		}

		putMsgToCache(r.cache, q.String(), msg)
		if r.mapping {
			ips := r.msgToIP(msg)
			for _, ip := range ips {
				putMsgToCache(r.cache, ip.String(), msg)
			}
		}
	}()

	ret, err, _ := r.group.Do(q.String(), func() (interface{}, error) {
		isIPReq := isIPRequest(q)
		if isIPReq {
			msg, err := r.fallbackExchange(m)
			return msg, err
		}

		return r.batchExchange(r.main, m)
	})

	if err == nil {
		msg = ret.(*D.Msg)
	}

	return
}

// IPToHost return fake-ip or redir-host mapping host
func (r *Resolver) IPToHost(ip net.IP) (string, bool) {
	if r.fakeip {
		return r.pool.LookBack(ip)
	}

	cache := r.cache.Get(ip.String())
	if cache == nil {
		return "", false
	}
	fqdn := cache.(*D.Msg).Question[0].Name
	return strings.TrimRight(fqdn, "."), true
}

func (r *Resolver) IsMapping() bool {
	return r.mapping
}

func (r *Resolver) IsFakeIP() bool {
	return r.fakeip
}

func (r *Resolver) batchExchange(clients []resolver, m *D.Msg) (msg *D.Msg, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fast, ctx := picker.WithContext(ctx)

	for _, client := range clients {
		r := client
		fast.Go(func() (interface{}, error) {
			msg, err := r.ExchangeContext(ctx, m)
			if err != nil || msg.Rcode != D.RcodeSuccess {
				return nil, errors.New("resolve error")
			}
			return msg, nil
		})
	}

	elm := fast.Wait()
	if elm == nil {
		return nil, errors.New("All DNS requests failed")
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
			if r.shouldFallback(ips[0]) {
				go func() { <-fallbackMsg }()
				msg = res.Msg
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
		}
	}

	query := &D.Msg{}
	query.SetQuestion(D.Fqdn(host), dnsType)

	msg, err := r.Exchange(query)
	if err != nil {
		return nil, err
	}

	ips := r.msgToIP(msg)
	if len(ips) == 0 {
		return nil, errIPNotFound
	}

	ip = ips[0]
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

func (r *Resolver) asyncExchange(client []resolver, msg *D.Msg) <-chan *result {
	ch := make(chan *result)
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
	IPv6           bool
	EnhancedMode   EnhancedMode
	FallbackFilter FallbackFilter
	Pool           *fakeip.Pool
}

func New(config Config) *Resolver {
	r := &Resolver{
		ipv6:    config.IPv6,
		main:    transform(config.Main),
		cache:   cache.New(time.Second * 60),
		mapping: config.EnhancedMode == MAPPING,
		fakeip:  config.EnhancedMode == FAKEIP,
		pool:    config.Pool,
	}

	if len(config.Fallback) != 0 {
		r.fallback = transform(config.Fallback)
	}

	fallbackFilters := []fallbackFilter{}
	if config.FallbackFilter.GeoIP {
		once.Do(func() {
			mmdb, _ = geoip2.Open(C.Path.MMDB())
		})

		fallbackFilters = append(fallbackFilters, &geoipFilter{})
	}
	for _, ipnet := range config.FallbackFilter.IPCIDR {
		fallbackFilters = append(fallbackFilters, &ipnetFilter{ipnet: ipnet})
	}
	r.fallbackFilters = fallbackFilters

	return r
}
