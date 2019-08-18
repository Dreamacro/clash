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
	tcpQueue  *channels.InfiniteChannel
	udpQueue  *channels.InfiniteChannel
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
	switch req.Metadata().NetWork {
	case C.TCP:
		t.tcpQueue.In() <- req
	case C.UDP:
		t.udpQueue.In() <- req
	}
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
	go func() {
		queue := t.udpQueue.Out()
		for {
			elm := <-queue
			conn := elm.(C.ServerAdapter)
			t.handleUDPConn(conn)
		}
	}()

	queue := t.tcpQueue.Out()
	for {
		elm := <-queue
		conn := elm.(C.ServerAdapter)
		go t.handleTCPConn(conn)
	}
}

func (t *Tunnel) resolveIP(host string) (net.IP, error) {
	return dns.ResolveIP(host)
}

func (t *Tunnel) needLookupIP(metadata *C.Metadata) bool {
	return dns.DefaultResolver != nil && (dns.DefaultResolver.IsMapping() || dns.DefaultResolver.IsFakeIP()) && metadata.Host == "" && metadata.DstIP != nil
}

func (t *Tunnel) checkMetadata(metadata *C.Metadata) bool {
	if !metadata.Valid() {
		return false
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
	return true
}

func (t *Tunnel) parseMetadata(metadata *C.Metadata) (C.Proxy, C.Rule, error) {
	var proxy C.Proxy
	var rule C.Rule
	switch t.mode {
	case Direct:
		proxy = t.proxies["DIRECT"]
	case Global:
		proxy = t.proxies["GLOBAL"]
	// Rule
	default:
		var err error
		proxy, rule, err = t.match(metadata)
		if err != nil {
			return nil, nil, err
		}
	}
	return proxy, rule, nil
}

func (t *Tunnel) handleUDPConn(localConn C.ServerAdapter) {
	metadata := localConn.Metadata()
	if ok := t.checkMetadata(metadata); !ok {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	src := localConn.RemoteAddr().String()
	dst := net.JoinHostPort(metadata.String(), metadata.DstPort)
	key := src + dst

	// Get from NAT table
	pc, addr := natTable.Get(key)
	if pc != nil {
		t.handleUDPToRemote(localConn, pc, addr)
		return
	}

	go func() {
		// Require WaitGroup from NAT pool
		wg, ok := natPool.Get(key)
		if ok {
			defer func() {
				wg.Done()
				natPool.Del(key)
			}()

			proxy, rule, err := t.parseMetadata(metadata)
			if err != nil {
				log.Warnln("Parse metadata failed: %v", err)
				return
			}

			rawPc, nAddr, err := proxy.DialUDP(metadata)
			addr = nAddr
			pc = rawPc
			if err != nil {
				log.Warnln("dial %s error: %s", proxy.Name(), err.Error())
				return
			}

			if rule != nil {
				log.Infoln("%s --> %v match %s using %s", metadata.SrcIP.String(), metadata.String(), rule.RuleType().String(), rawPc.Chains().String())
			} else {
				log.Infoln("%s --> %v doesn't match any rule using DIRECT", metadata.SrcIP.String(), metadata.String())
			}

			natTable.Add(key, pc, addr, func() {
				t.handleUDPToLocal(localConn, pc, udpTimeout)
			})

			t.handleUDPToRemote(localConn, pc, addr)
		} else {
			// Wait for the first UDP packet process finished
			wg.Wait()

			pc, addr = natTable.Get(key)
			if pc != nil {
				t.handleUDPToRemote(localConn, pc, addr)
			}
		}
	}()
}

func (t *Tunnel) handleTCPConn(localConn C.ServerAdapter) {
	defer localConn.Close()

	metadata := localConn.Metadata()
	if ok := t.checkMetadata(metadata); !ok {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	proxy, rule, err := t.parseMetadata(metadata)
	if err != nil {
		log.Warnln("Parse metadata failed: %v", err)
		return
	}

	remoteConn, err := proxy.Dial(metadata)
	if err != nil {
		log.Warnln("dial %s error: %s", proxy.Name(), err.Error())
		return
	}
	defer remoteConn.Close()

	if rule != nil {
		log.Infoln("%s --> %v match %s using %s", metadata.SrcIP.String(), metadata.String(), rule.RuleType().String(), remoteConn.Chains().String())
	} else {
		log.Infoln("%s --> %v doesn't match any rule using DIRECT", metadata.SrcIP.String(), metadata.String())
	}

	switch adapter := localConn.(type) {
	case *InboundAdapter.HTTPAdapter:
		t.handleHTTP(adapter, remoteConn)
	case *InboundAdapter.SocketAdapter:
		t.handleSocket(adapter, remoteConn)
	}
}

func (t *Tunnel) shouldResolveIP(rule C.Rule, metadata *C.Metadata) bool {
	return (rule.RuleType() == C.GEOIP || rule.RuleType() == C.IPCIDR) && metadata.Host != "" && metadata.DstIP == nil
}

func (t *Tunnel) match(metadata *C.Metadata) (C.Proxy, C.Rule, error) {
	t.configMux.RLock()
	defer t.configMux.RUnlock()

	var resolved bool
	for _, rule := range t.rules {
		if !resolved && t.shouldResolveIP(rule, metadata) {
			ip, err := t.resolveIP(metadata.Host)
			if err != nil {
				if !t.ignoreResolveFail {
					return nil, nil, fmt.Errorf("[DNS] resolve %s error: %s", metadata.Host, err.Error())
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
			return adapter, rule, nil
		}
	}
	return t.proxies["DIRECT"], nil, nil
}

func newTunnel() *Tunnel {
	return &Tunnel{
		tcpQueue:  channels.NewInfiniteChannel(),
		udpQueue:  channels.NewInfiniteChannel(),
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
