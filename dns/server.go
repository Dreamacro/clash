package dns

import (
	"errors"
	"net"
	"strings"

	"github.com/Dreamacro/clash/common/sockopt"
	"github.com/Dreamacro/clash/context"
	"github.com/Dreamacro/clash/log"

	D "github.com/miekg/dns"
)

var (
	address string
	server  = &Server{}

	dnsDefaultTTL uint32 = 600
)

type Server struct {
	*D.Server
	handler handler
}

func toString(rr D.RR) string {
	switch t := rr.(type) {
	case *D.A:
		return "A:" + t.A.String()
	case *D.AAAA:
		return "AAAA:" + t.AAAA.String()
	case *D.CNAME:
		return "CNAME:" + t.Target
	default:
		return rr.String()
	}
}

// ServeDNS implement D.Handler ServeDNS
func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
	msg, err := handlerWithContext(s.handler, r)
	if err != nil {
		D.HandleFailed(w, r)
		return
	}
	var answer []string
	for _, rr := range msg.Answer {
		answer = append(answer, toString(rr))
	}
	log.Debugln("Served DNS: %v -> %v", msg.Question[0].Name, strings.Join(answer, ", "))
	w.WriteMsg(msg)
}

func handlerWithContext(handler handler, msg *D.Msg) (*D.Msg, error) {
	if len(msg.Question) == 0 {
		return nil, errors.New("at least one question is required")
	}

	ctx := context.NewDNSContext(msg)
	return handler(ctx, msg)
}

func (s *Server) setHandler(handler handler) {
	s.handler = handler
}

func ReCreateServer(addr string, resolver *Resolver, mapper *ResolverEnhancer) error {
	if addr == address && resolver != nil {
		handler := newHandler(resolver, mapper)
		server.setHandler(handler)
		return nil
	}

	if server.Server != nil {
		server.Shutdown()
		server = &Server{}
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

	err = sockopt.UDPReuseaddr(p)
	if err != nil {
		log.Warnln("Failed to Reuse UDP Address: %s", err)
	}

	address = addr
	handler := newHandler(resolver, mapper)
	server = &Server{handler: handler}
	server.Server = &D.Server{Addr: addr, PacketConn: p, Handler: server}

	go func() {
		server.ActivateAndServe()
	}()
	return nil
}
