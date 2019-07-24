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

// Tunnel handle relay inbound proxy and outbound proxy
type Tunnel struct {
	queue     *channels.InfiniteChannel
	rules     []C.Rule
	proxies   map[string]C.Proxy
	configMux *sync.RWMutex
	traffic   *C.Traffic

	// experimental features
	ignoreResolveFail bool

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
	t.configMux.Lock()
	t.rules = rules
	t.configMux.Unlock()
}

// Proxies return all proxies
func (t *Tunnel) Proxies() map[string]C.Proxy {
	return t.proxies
}

// UpdateProxies handle update proxies
func (t *Tunnel) UpdateProxies(proxies map[string]C.Proxy) {
	t.configMux.Lock()
	t.proxies = proxies
	t.configMux.Unlock()
}

// UpdateExperimental handle update experimental config
func (t *Tunnel) UpdateExperimental(ignoreResolveFail bool) {
	t.configMux.Lock()
	t.ignoreResolveFail = ignoreResolveFail
	t.configMux.Unlock()
}

// Mode return current mode
func (t *Tunnel) Mode() Mode {
	return t.mode
}

// SetMode change the mode of tunnel
func (t *Tunnel) SetMode(mode Mode) {
	t.mode = mode
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
	return dns.ResolveIP(host)
}

func (t *Tunnel) needLookupIP(metadata *C.Metadata) bool {
	return dns.DefaultResolver != nil && (dns.DefaultResolver.IsMapping() || dns.DefaultResolver.IsFakeIP()) && metadata.Host == "" && metadata.DstIP != nil
}

func (t *Tunnel) handleConn(localConn C.ServerAdapter) {
	defer func() {
		var conn net.Conn
		switch adapter := localConn.(type) {
		case *InboundAdapter.HTTPAdapter:
			conn = adapter.Conn
		case *InboundAdapter.SocketAdapter:
			conn = adapter.Conn
		}
		if _, ok := conn.(*net.TCPConn); ok {
			localConn.Close()
		}
	}()

	metadata := localConn.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	// preprocess enhanced-mode metadata
	if t.needLookupIP(metadata) {
		host, exist := dns.DefaultResolver.IPToHost(*metadata.DstIP)
		if exist {
			metadata.Host = host
			metadata.AddrType = C.AtypDomainName
			if dns.DefaultResolver.IsFakeIP() {
				metadata.DstIP = nil
			}
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

	switch metadata.NetWork {
	case C.TCP:
		t.handleTCPConn(localConn, metadata, proxy)
	case C.UDP:
		t.handleUDPConn(localConn, metadata, proxy)
	}
}

func (t *Tunnel) handleUDPConn(localConn C.ServerAdapter, metadata *C.Metadata, proxy C.Proxy) {
	pc, addr := natTable.Get(localConn.RemoteAddr())
	if pc == nil {
		var err error
		pc, addr, err = proxy.DialUDP(metadata)
		if err != nil {
			log.Warnln("Proxy[%s] connect [%s --> %s] error: %s", proxy.Name(), metadata.SrcIP.String(), metadata.String(), err.Error())
			return
		}

		natTable.Set(localConn.RemoteAddr(), pc, addr)
		go t.handleUDPToLocal(localConn, pc)
	}

	t.handleUDPToRemote(localConn, pc, addr)
}

func (t *Tunnel) handleTCPConn(localConn C.ServerAdapter, metadata *C.Metadata, proxy C.Proxy) {
	remoConn, err := proxy.Dial(metadata)
	if err != nil {
		log.Warnln("Proxy[%s] connect [%s --> %s] error: %s", proxy.Name(), metadata.SrcIP.String(), metadata.String(), err.Error())
		return
	}
	defer remoConn.Close()

	switch adapter := localConn.(type) {
	case *InboundAdapter.HTTPAdapter:
		t.handleHTTP(adapter, remoConn)
	case *InboundAdapter.SocketAdapter:
		t.handleSocket(adapter, remoConn)
	}
}

func (t *Tunnel) shouldResolveIP(rule C.Rule, metadata *C.Metadata) bool {
	return (rule.RuleType() == C.GEOIP || rule.RuleType() == C.IPCIDR) && metadata.Host != "" && metadata.DstIP == nil
}

func (t *Tunnel) match(metadata *C.Metadata) (C.Proxy, error) {
	t.configMux.RLock()
	defer t.configMux.RUnlock()

	var resolved bool
	for _, rule := range t.rules {
		if !resolved && t.shouldResolveIP(rule, metadata) {
			ip, err := t.resolveIP(metadata.Host)
			if err != nil {
				if !t.ignoreResolveFail {
					return nil, fmt.Errorf("[DNS] resolve %s error: %s", metadata.Host, err.Error())
				}
				log.Debugln("[DNS] resolve %s error: %s", metadata.Host, err.Error())
			} else {
				log.Debugln("[DNS] %s --> %s", metadata.Host, ip.String())
				metadata.DstIP = &ip
			}
			resolved = true
		}

		if rule.IsMatch(metadata) {
			adapter, ok := t.proxies[rule.Adapter()]
			if !ok {
				continue
			}

			if metadata.NetWork == C.UDP && !adapter.SupportUDP() {
				log.Debugln("%v UDP is not supported", adapter.Name())
				continue
			}

			log.Infoln("%s --> %v match %s using %s", metadata.SrcIP.String(), metadata.String(), rule.RuleType().String(), rule.Adapter())
			return adapter, nil
		}
	}
	log.Infoln("%s --> %v doesn't match any rule using DIRECT", metadata.SrcIP.String(), metadata.String())
	return t.proxies["DIRECT"], nil
}

func newTunnel() *Tunnel {
	return &Tunnel{
		queue:     channels.NewInfiniteChannel(),
		proxies:   make(map[string]C.Proxy),
		configMux: &sync.RWMutex{},
		traffic:   C.NewTraffic(time.Second),
		mode:      Rule,
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
