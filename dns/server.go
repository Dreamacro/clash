package dns

import (
	"fmt"
	"net"

	"github.com/Dreamacro/clash/log"
	D "github.com/miekg/dns"
)

var (
	address string
	server  = &Server{}
)

type Server struct {
	*D.Server
	r *Resolver
}

func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
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

func ReCreateServer(addr string, resolver *Resolver) error {
	if server.Server != nil {
		server.Shutdown()
	}

	if addr == address {
		return nil
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
