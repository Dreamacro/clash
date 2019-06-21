package dns

import (
	"errors"
	"fmt"
	"net"

	"github.com/Dreamacro/clash/log"
	"github.com/miekg/dns"
	D "github.com/miekg/dns"
)

var (
	address string
	server  = &Server{}

	dnsDefaultTTL uint32 = 600
)

type Server struct {
	*D.Server
	r *Resolver
}

func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
	if s.r.IsFakeIP() {
		msg, err := s.handleFakeIP(r)
		if err != nil {
			D.HandleFailed(w, r)
			return
		}
		msg.SetReply(r)
		w.WriteMsg(msg)
		return
	}

	msg, err := s.r.Exchange(r)

	if err != nil {
		if len(r.Question) > 0 {
			q := r.Question[0]
			qString := fmt.Sprintf("%s %s %s", q.Name, D.Class(q.Qclass).String(), D.Type(q.Qtype).String())
			log.Debugln("[DNS Server] Exchange %s failed: %v", qString, err)
		}
		D.HandleFailed(w, r)
		return
	}
	msg.SetReply(r)
	w.WriteMsg(msg)
}

func (s *Server) handleFakeIP(r *D.Msg) (msg *D.Msg, err error) {
	if len(r.Question) == 0 {
		err = errors.New("should have one question at least")
		return
	}

	q := r.Question[0]

	cache := s.r.cache.Get("fakeip:" + q.String())
	if cache != nil {
		msg = cache.(*D.Msg).Copy()
		setMsgTTL(msg, 1)
		return
	}

	var ip net.IP
	defer func() {
		if msg == nil {
			return
		}

		putMsgToCache(s.r.cache, "fakeip:"+q.String(), msg)
		putMsgToCache(s.r.cache, ip.String(), msg)

		setMsgTTL(msg, 1)
	}()

	rr := &D.A{}
	rr.Hdr = dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: dnsDefaultTTL}
	ip = s.r.pool.Get()
	rr.A = ip
	msg = r.Copy()
	msg.Answer = []D.RR{rr}
	return
}

func (s *Server) setReslover(r *Resolver) {
	s.r = r
}

func ReCreateServer(addr string, resolver *Resolver) error {
	if addr == address {
		server.setReslover(resolver)
		return nil
	}

	if server.Server != nil {
		server.Shutdown()
		address = ""
	}

	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return nil
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	p, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	address = addr
	server = &Server{r: resolver}
	server.Server = &D.Server{Addr: addr, PacketConn: p, Handler: server}

	go func() {
		server.ActivateAndServe()
	}()
	return nil
}
