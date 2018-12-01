package tunnel

import (
	"net"
	"sync"
	"time"

	InboundAdapter "github.com/Dreamacro/clash/adapters/inbound"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"

	"gopkg.in/eapache/channels.v1"
)

var (
	tunnel *Tunnel
	once   sync.Once
)

// Tunnel handle proxy socket and HTTP/SOCKS socket
type Tunnel struct {
	queue      *channels.InfiniteChannel
	rules      []C.Rule
	proxies    map[string]C.Proxy
	configLock *sync.RWMutex
	traffic    *C.Traffic
	resolver   *dns.Resolver

	// Outbound Rule
	mode Mode
}

// Add request to queue
func (t *Tunnel) Add(req C.ServerAdapter) {
	t.queue.In() <- req
}

// Traffic return traffic of all connections
func (t *Tunnel) Traffic() *C.Traffic {
	return t.traffic
}

// Rules return all rules
func (t *Tunnel) Rules() []C.Rule {
	return t.rules
}

// UpdateRules handle update rules
func (t *Tunnel) UpdateRules(rules []C.Rule) {
	t.configLock.Lock()
	t.rules = rules
	t.configLock.Unlock()
}

// Proxies return all proxies
func (t *Tunnel) Proxies() map[string]C.Proxy {
	return t.proxies
}

// UpdateProxies handle update proxies
func (t *Tunnel) UpdateProxies(proxies map[string]C.Proxy) {
	t.configLock.Lock()
	t.proxies = proxies
	t.configLock.Unlock()
}

// Mode return current mode
func (t *Tunnel) Mode() Mode {
	return t.mode
}

// SetMode change the mode of tunnel
func (t *Tunnel) SetMode(mode Mode) {
	t.mode = mode
}

// SetResolver change the resolver of tunnel
func (t *Tunnel) SetResolver(resolver *dns.Resolver) {
	t.resolver = resolver
}

func (t *Tunnel) process() {
	queue := t.queue.Out()
	for {
		elm := <-queue
		conn := elm.(C.ServerAdapter)
		go t.handleConn(conn)
	}
}

func (t *Tunnel) resolveIP(host string) (net.IP, error) {
	if t.resolver == nil {
		ipAddr, err := net.ResolveIPAddr("ip", host)
		if err != nil {
			return nil, err
		}

		return ipAddr.IP, nil
	}

	return t.resolver.ResolveIP(host)
}

func (t *Tunnel) handleConn(localConn C.ServerAdapter) {
	defer localConn.Close()
	metadata := localConn.Metadata()

	if metadata.Source == C.REDIR && t.resolver != nil {
		host, exist := t.resolver.IPToHost(*metadata.IP)
		if exist {
			metadata.Host = host
			metadata.AddrType = C.AtypDomainName
		}
	} else if metadata.IP == nil && metadata.AddrType == C.AtypDomainName {
		ip, err := t.resolveIP(metadata.Host)
		if err != nil {
			log.Debugln("[DNS] resolve %s error: %s", metadata.Host, err.Error())
		} else {
			log.Debugln("[DNS] %s --> %s", metadata.Host, ip.String())
			metadata.IP = &ip
		}
	} else {
		log.Debugln("[DNS] unknown%#v", metadata)
	}

	var proxy C.Proxy
	switch t.mode {
	case Direct:
		proxy = t.proxies["DIRECT"]
	case Global:
		proxy = t.proxies["GLOBAL"]
	// Rule
	default:
		proxy = t.match(metadata)
	}
	remoConn, err := proxy.Generator(metadata)
	if err != nil {
		log.Warnln("Proxy[%s] connect [%s] error: %s", proxy.Name(), metadata.String(), err.Error())
		return
	}
	defer remoConn.Close()

	switch adapter := localConn.(type) {
	case *InboundAdapter.HTTPAdapter:
		t.handleHTTP(adapter, remoConn)
	case *InboundAdapter.SocketAdapter:
		t.handleSOCKS(adapter, remoConn)
	}
}

func (t *Tunnel) match(metadata *C.Metadata) C.Proxy {
	t.configLock.RLock()
	defer t.configLock.RUnlock()

	for _, rule := range t.rules {
		if rule.IsMatch(metadata) {
			a, ok := t.proxies[rule.Adapter()]
			if !ok {
				continue
			}
			log.Infoln("%v match %s using %s", metadata.String(), rule.RuleType().String(), rule.Adapter())
			return a
		}
	}
	log.Infoln("%v doesn't match any rule using DIRECT", metadata.String())
	return t.proxies["DIRECT"]
}

func newTunnel() *Tunnel {
	return &Tunnel{
		queue:      channels.NewInfiniteChannel(),
		proxies:    make(map[string]C.Proxy),
		configLock: &sync.RWMutex{},
		traffic:    C.NewTraffic(time.Second),
		mode:       Rule,
	}
}

// Instance return singleton instance of Tunnel
func Instance() *Tunnel {
	once.Do(func() {
		tunnel = newTunnel()
		go tunnel.process()
	})
	return tunnel
}
