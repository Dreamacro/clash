package dns

import (
	"fmt"
	"net"
	"strings"

	"github.com/Dreamacro/clash/component/fakeip"
	"github.com/Dreamacro/clash/log"

	D "github.com/miekg/dns"
)

type handler func(w D.ResponseWriter, r *D.Msg)

func withFakeIP(pool *fakeip.Pool) handler {
	return func(w D.ResponseWriter, r *D.Msg) {
		q := r.Question[0]
		host := strings.TrimRight(q.Name, ".")

		rr := &D.A{}
		rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
		ip := pool.Lookup(host)
		rr.A = ip
		msg := r.Copy()
		msg.Answer = []D.RR{rr}

		setMsgTTL(msg, 1)
		msg.SetReply(r)
		w.WriteMsg(msg)
		return
	}
}

func withResolver(resolver *Resolver) handler {
	return func(w D.ResponseWriter, r *D.Msg) {
		msg, err := resolver.Exchange(r)

		if err != nil {
			q := r.Question[0]
			qString := fmt.Sprintf("%s %s %s", q.Name, D.Class(q.Qclass).String(), D.Type(q.Qtype).String())
			log.Debugln("[DNS Server] Exchange %s failed: %v", qString, err)
			D.HandleFailed(w, r)
			return
		}
		msg.SetReply(r)
		w.WriteMsg(msg)
		return
	}
}

func withHost(resolver *Resolver, next handler) handler {
	hosts := resolver.hosts
	if hosts == nil {
		panic("dns/withHost: hosts should not be nil")
	}

	return func(w D.ResponseWriter, r *D.Msg) {
		q := r.Question[0]
		if q.Qtype != D.TypeA && q.Qtype != D.TypeAAAA {
			next(w, r)
			return
		}

		domain := strings.TrimRight(q.Name, ".")
		host := hosts.Search(domain)
		if host == nil {
			next(w, r)
			return
		}

		ip := host.Data.(net.IP)
		if q.Qtype == D.TypeAAAA && ip.To16() == nil {
			next(w, r)
			return
		} else if q.Qtype == D.TypeA && ip.To4() == nil {
			next(w, r)
			return
		}

		var rr D.RR
		if q.Qtype == D.TypeAAAA {
			record := &D.AAAA{}
			record.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeAAAA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
			record.AAAA = ip
			rr = record
		} else {
			record := &D.A{}
			record.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
			record.A = ip
			rr = record
		}

		msg := r.Copy()
		msg.Answer = []D.RR{rr}
		msg.SetReply(r)
		w.WriteMsg(msg)
		return
	}
}

func newHandler(resolver *Resolver) handler {
	if resolver.IsFakeIP() {
		return withFakeIP(resolver.pool)
	}

	if resolver.hosts != nil {
		return withHost(resolver, withResolver(resolver))
	}

	return withResolver(resolver)
}
