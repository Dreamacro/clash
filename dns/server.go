package dns

import (
	"net"

	"github.com/Dreamacro/clash/common/sockopt"

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

func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
	if len(r.Question) == 0 {
		D.HandleFailed(w, r)
		return
	}

	s.handler(w, r)
}

func (s *Server) setHandler(handler handler) {
	s.handler = handler
}

func ReCreateServer(addr string, resolver *Resolver) error {
	if addr == address && resolver != nil {
		handler := newHandler(resolver)
		server.setHandler(handler)
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

	err = sockopt.UDPReuseaddr(p)
	if err != nil {
		return err
	}

	address = addr
	handler := newHandler(resolver)
	server = &Server{handler: handler}
	server.Server = &D.Server{Addr: addr, PacketConn: p, Handler: server}

	go func() {
		server.ActivateAndServe()
	}()
	return nil
}
