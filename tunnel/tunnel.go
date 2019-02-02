package tunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	InboundAdapter "github.com/Dreamacro/clash/adapters/inbound"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"

	channels "gopkg.in/eapache/channels.v1"
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

func (t *Tunnel) hasResolver() bool {
	return t.resolver != nil
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

	if metadata.NeedLoopUpHost() && t.hasResolver() {
		host, exist := t.resolver.IPToHost(*metadata.IP)
		if exist {
			metadata.Host = host
			metadata.AddrType = C.AtypDomainName
		}
	}

	var proxy C.Proxy
	switch t.mode {
	case Direct:
		proxy = t.proxies["DIRECT"]
	case Global:
		proxy = t.proxies["GLOBAL"]
	// Rule
	default:
		var err error
		proxy, err = t.match(metadata)
		if err != nil {
			return
		}
	}

	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	remoConn, err := proxy.Generator(metadata)
	if err != nil {
		log.Warnln("Proxy[%s] connect [%s --> %s] error: %s", proxy.Name(), metadata.SourceIP.String(), metadata.String(), err.Error())
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

func (t *Tunnel) shouldResolveIP(rule C.Rule, metadata *C.Metadata) bool {
	return (rule.RuleType() == C.GEOIP || rule.RuleType() == C.IPCIDR) && metadata.Host != "" && metadata.IP == nil
}

func (t *Tunnel) match(metadata *C.Metadata) (C.Proxy, error) {
	t.configLock.RLock()
	defer t.configLock.RUnlock()

	for _, rule := range t.rules {
		if t.shouldResolveIP(rule, metadata) {
			ip, err := t.resolveIP(metadata.Host)
			if err != nil {
				return nil, fmt.Errorf("[DNS] resolve %s error: %s", metadata.Host, err.Error())
			}
			log.Debugln("[DNS] %s --> %s", metadata.Host, ip.String())
			metadata.IP = &ip
		}

		if rule.IsMatch(metadata) {
			if a, ok := t.proxies[rule.Adapter()]; ok {
				log.Infoln("%s --> %v match %s using %s", metadata.SourceIP.String(), metadata.String(), rule.RuleType().String(), rule.Adapter())
				return a, nil
			}
		}
	}
	log.Infoln("%s --> %v doesn't match any rule using DIRECT", metadata.SourceIP.String(), metadata.String())
	return t.proxies["DIRECT"], nil
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
