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
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"

	D "github.com/miekg/dns"
	geoip2 "github.com/oschwald/geoip2-golang"
)

var (
	globalSessionCache = tls.NewLRUClientSessionCache(64)

	mmdb     *geoip2.Reader
	once     sync.Once
	resolver *Resolver
)

type Resolver struct {
	ipv6     bool
	mapping  bool
	fallback []*nameserver
	main     []*nameserver
	cache    *cache.Cache
}

type result struct {
	Msg   *D.Msg
	Error error
}

func isIPRequest(q D.Question) bool {
	if q.Qclass == D.ClassINET && (q.Qtype == D.TypeA || q.Qtype == D.TypeAAAA) {
		return true
	}
	return false
}

func (r *Resolver) Exchange(m *D.Msg) (msg *D.Msg, err error) {
	if len(m.Question) == 0 {
		return nil, errors.New("should have one question at least")
	}

	q := m.Question[0]
	cache := r.cache.Get(q.String())
	if cache != nil {
		return cache.(*D.Msg).Copy(), nil
	}
	defer func() {
		if msg != nil {
			putMsgToCache(r.cache, q.String(), msg)
			if r.mapping {
				ips, err := r.msgToIP(msg)
				if err != nil {
					log.Debugln("[DNS] msg to ip error: %s", err.Error())
					return
				}
				for _, ip := range ips {
					putMsgToCache(r.cache, ip.String(), msg)
				}
			}
		}
	}()

	isIPReq := isIPRequest(q)
	if isIPReq {
		msg, err = r.resolveIP(m)
		return
	}

	msg, err = r.exchange(r.main, m)
	return
}

func (r *Resolver) exchange(servers []*nameserver, m *D.Msg) (msg *D.Msg, err error) {
	in := make(chan interface{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fast := picker.SelectFast(ctx, in)

	wg := sync.WaitGroup{}
	wg.Add(len(servers))
	for _, server := range servers {
		go func(s *nameserver) {
			defer wg.Done()
			msg, _, err := s.Client.Exchange(m, s.Address)
			if err != nil || msg.Rcode != D.RcodeSuccess {
				return
			}
			in <- &result{Msg: msg, Error: err}
		}(server)
	}

	// release in channel
	go func() {
		wg.Wait()
		close(in)
	}()

	elm, exist := <-fast
	if !exist {
		return nil, errors.New("All DNS requests failed")
	}

	resp := elm.(*result)
	msg, err = resp.Msg, resp.Error
	return
}

func (r *Resolver) resolveIP(m *D.Msg) (msg *D.Msg, err error) {
	msgCh := r.resolve(r.main, m)
	if r.fallback == nil {
		res := <-msgCh
		msg, err = res.Msg, res.Error
		return
	}
	fallbackMsg := r.resolve(r.fallback, m)
	res := <-msgCh
	if res.Error == nil {
		if mmdb == nil {
			return nil, errors.New("GeoIP can't use")
		}

		ips, _ := r.msgToIP(res.Msg)
		if record, _ := mmdb.Country(ips[0]); record.Country.IsoCode == "CN" || record.Country.IsoCode == "" {
			// release channel
			go func() { <-fallbackMsg }()
			msg = res.Msg
			return
		}
	}

	res = <-fallbackMsg
	msg, err = res.Msg, res.Error
	return
}

func (r *Resolver) ResolveIP(host string) (ip net.IP, err error) {
	query := &D.Msg{}
	dnsType := D.TypeA
	if r.ipv6 {
		dnsType = D.TypeAAAA
	}
	query.SetQuestion(D.Fqdn(host), dnsType)

	msg, err := r.Exchange(query)
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	ips, err = r.msgToIP(msg)
	if err != nil {
		return nil, err
	}

	ip = ips[0]
	return
}

func (r *Resolver) msgToIP(msg *D.Msg) ([]net.IP, error) {
	var ips []net.IP

	for _, answer := range msg.Answer {
		if r.ipv6 {
			ans, ok := answer.(*D.AAAA)
			if !ok {
				continue
			}
			ips = append(ips, ans.AAAA)
			continue
		}

		ans, ok := answer.(*D.A)
		if !ok {
			continue
		}
		ips = append(ips, ans.A)
	}

	if len(ips) == 0 {
		return nil, errors.New("Can't parse msg")
	}

	return ips, nil
}

func (r *Resolver) IPToHost(ip net.IP) (string, bool) {
	cache := r.cache.Get(ip.String())
	if cache == nil {
		return "", false
	}
	fqdn := cache.(*D.Msg).Question[0].Name
	return strings.TrimRight(fqdn, "."), true
}

func (r *Resolver) resolve(client []*nameserver, msg *D.Msg) <-chan *result {
	ch := make(chan *result)
	go func() {
		res, err := r.exchange(client, msg)
		ch <- &result{Msg: res, Error: err}
	}()
	return ch
}

type NameServer struct {
	Net  string
	Addr string
}

type nameserver struct {
	Client  *D.Client
	Address string
}

type Config struct {
	Main, Fallback []NameServer
	IPv6           bool
	EnhancedMode   EnhancedMode
}

func transform(servers []NameServer) []*nameserver {
	var ret []*nameserver
	for _, s := range servers {
		ret = append(ret, &nameserver{
			Client: &D.Client{
				Net: s.Net,
				TLSConfig: &tls.Config{
					ClientSessionCache: globalSessionCache,
				},
			},
			Address: s.Addr,
		})
	}
	return ret
}

func New(config Config) *Resolver {
	once.Do(func() {
		mmdb, _ = geoip2.Open(C.Path.MMDB())
	})

	r := &Resolver{
		main:    transform(config.Main),
		ipv6:    config.IPv6,
		cache:   cache.New(time.Second * 60),
		mapping: config.EnhancedMode == MAPPING,
	}
	if config.Fallback != nil {
		r.fallback = transform(config.Fallback)
	}
	return r
}
