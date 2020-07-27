package tunnel

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/component/nat"
	"github.com/Dreamacro/clash/component/resolver"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"

	channels "gopkg.in/eapache/channels.v1"
)

var (
	tcpQueue     = channels.NewInfiniteChannel()
	udpQueue     = channels.NewInfiniteChannel()
	natTable     = nat.New()
	rules        []C.Rule
	proxies      = make(map[string]C.Proxy)
	providers    map[string]provider.ProxyProvider
	configMux    sync.RWMutex
	enhancedMode *dns.Resolver

	// Outbound Rule
	mode = Rule

	// default timeout for UDP session
	udpTimeout = 60 * time.Second
)

func init() {
	go process()
}

// Add request to queue
func Add(req C.ServerAdapter) {
	tcpQueue.In() <- req
}

// AddPacket add udp Packet to queue
func AddPacket(packet *inbound.PacketAdapter) {
	udpQueue.In() <- packet
}

// Rules return all rules
func Rules() []C.Rule {
	return rules
}

// UpdateRules handle update rules
func UpdateRules(newRules []C.Rule) {
	configMux.Lock()
	rules = newRules
	configMux.Unlock()
}

// Proxies return all proxies
func Proxies() map[string]C.Proxy {
	return proxies
}

// Providers return all compatible providers
func Providers() map[string]provider.ProxyProvider {
	return providers
}

// UpdateProxies handle update proxies
func UpdateProxies(newProxies map[string]C.Proxy, newProviders map[string]provider.ProxyProvider) {
	configMux.Lock()
	proxies = newProxies
	providers = newProviders
	configMux.Unlock()
}

// Mode return current mode
func Mode() TunnelMode {
	return mode
}

// SetMode change the mode of tunnel
func SetMode(m TunnelMode) {
	mode = m
}

// SetResolver set custom dns resolver for enhanced mode
func SetResolver(r *dns.Resolver) {
	enhancedMode = r
}

// processUDP starts a loop to handle udp packet
func processUDP() {
	queue := udpQueue.Out()
	for elm := range queue {
		conn := elm.(*inbound.PacketAdapter)
		handleUDPConn(conn)
	}
}

func process() {
	numUDPWorkers := 4
	if runtime.NumCPU() > numUDPWorkers {
		numUDPWorkers = runtime.NumCPU()
	}
	for i := 0; i < numUDPWorkers; i++ {
		go processUDP()
	}

	queue := tcpQueue.Out()
	for elm := range queue {
		conn := elm.(C.ServerAdapter)
		go handleTCPConn(conn)
	}
}

func needLookupIP(metadata *C.Metadata) bool {
	return enhancedMode != nil && (enhancedMode.IsMapping() || enhancedMode.FakeIPEnabled()) && metadata.Host == "" && metadata.DstIP != nil
}

func preHandleMetadata(metadata *C.Metadata) error {
	// handle IP string on host
	if ip := net.ParseIP(metadata.Host); ip != nil {
		metadata.DstIP = ip
	}

	// preprocess enhanced-mode metadata
	if needLookupIP(metadata) {
		host, exist := enhancedMode.IPToHost(metadata.DstIP)
		if exist {
			metadata.Host = host
			metadata.AddrType = C.AtypDomainName
			if enhancedMode.FakeIPEnabled() {
				metadata.DstIP = nil
			} else if node := resolver.DefaultHosts.Search(host); node != nil {
				// redir-host should lookup the hosts
				metadata.DstIP = node.Data.(net.IP)
			}
		} else if enhancedMode.IsFakeIP(metadata.DstIP) {
			return fmt.Errorf("fake DNS record %s missing", metadata.DstIP)
		}
	}

	return nil
}

func resolveMetadata(metadata *C.Metadata) (C.Proxy, C.Rule, error) {
	var proxy C.Proxy
	var rule C.Rule
	switch mode {
	case Direct:
		proxy = proxies["DIRECT"]
	case Global:
		proxy = proxies["GLOBAL"]
	// Rule
	default:
		var err error
		proxy, rule, err = match(metadata)
		if err != nil {
			return nil, nil, err
		}
	}
	return proxy, rule, nil
}

func handleUDPConn(packet *inbound.PacketAdapter) {
	metadata := packet.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	// make a fAddr if requset ip is fakeip
	var fAddr net.Addr
	if enhancedMode != nil && enhancedMode.IsFakeIP(metadata.DstIP) {
		fAddr = metadata.UDPAddr()
	}

	if err := preHandleMetadata(metadata); err != nil {
		log.Debugln("[Metadata PreHandle] error: %s", err)
		return
	}

	key := packet.LocalAddr().String()
	pc := natTable.Get(key)
	if pc != nil {
		handleUDPToRemote(packet, pc, metadata)
		return
	}

	lockKey := key + "-lock"
	wg, loaded := natTable.GetOrCreateLock(lockKey)

	go func() {
		if !loaded {
			wg.Add(1)
			proxy, rule, err := resolveMetadata(metadata)
			if err != nil {
				log.Warnln("[UDP] Parse metadata failed: %s", err.Error())
				natTable.Delete(lockKey)
				wg.Done()
				return
			}

			rawPc, err := proxy.DialUDP(metadata)
			if err != nil {
				log.Warnln("[UDP] dial %s error: %s", proxy.Name(), err.Error())
				natTable.Delete(lockKey)
				wg.Done()
				return
			}
			pc = newUDPTracker(rawPc, DefaultManager, metadata, rule)

			switch true {
			case rule != nil:
				log.Infoln("[UDP] %s --> %v match %s(%s) using %s", metadata.SourceAddress(), metadata.String(), rule.RuleType().String(), rule.Payload(), rawPc.Chains().String())
			case mode == Global:
				log.Infoln("[UDP] %s --> %v using GLOBAL", metadata.SourceAddress(), metadata.String())
			case mode == Direct:
				log.Infoln("[UDP] %s --> %v using DIRECT", metadata.SourceAddress(), metadata.String())
			default:
				log.Infoln("[UDP] %s --> %v doesn't match any rule using DIRECT", metadata.SourceAddress(), metadata.String())
			}

			natTable.Set(key, pc)
			natTable.Delete(lockKey)
			wg.Done()
			go handleUDPToLocal(packet.UDPPacket, pc, key, fAddr)
		}

		wg.Wait()
		pc := natTable.Get(key)
		if pc != nil {
			handleUDPToRemote(packet, pc, metadata)
		}
	}()
}

func handleTCPConn(localConn C.ServerAdapter) {
	defer localConn.Close()

	metadata := localConn.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	if err := preHandleMetadata(metadata); err != nil {
		log.Debugln("[Metadata PreHandle] error: %s", err)
		return
	}

	proxy, rule, err := resolveMetadata(metadata)
	if err != nil {
		log.Warnln("Parse metadata failed: %v", err)
		return
	}

	remoteConn, err := proxy.Dial(metadata)
	if err != nil {
		log.Warnln("dial %s error: %s", proxy.Name(), err.Error())
		return
	}
	remoteConn = newTCPTracker(remoteConn, DefaultManager, metadata, rule)
	defer remoteConn.Close()

	switch true {
	case rule != nil:
		log.Infoln("[TCP] %s --> %v match %s(%s) using %s", metadata.SourceAddress(), metadata.String(), rule.RuleType().String(), rule.Payload(), remoteConn.Chains().String())
	case mode == Global:
		log.Infoln("[TCP] %s --> %v using GLOBAL", metadata.SourceAddress(), metadata.String())
	case mode == Direct:
		log.Infoln("[TCP] %s --> %v using DIRECT", metadata.SourceAddress(), metadata.String())
	default:
		log.Infoln("[TCP] %s --> %v doesn't match any rule using DIRECT", metadata.SourceAddress(), metadata.String())
	}

	switch adapter := localConn.(type) {
	case *inbound.HTTPAdapter:
		handleHTTP(adapter, remoteConn)
	case *inbound.SocketAdapter:
		handleSocket(adapter, remoteConn)
	}
}

func shouldResolveIP(rule C.Rule, metadata *C.Metadata) bool {
	return rule.ShouldResolveIP() && metadata.Host != "" && metadata.DstIP == nil
}

func match(metadata *C.Metadata) (C.Proxy, C.Rule, error) {
	configMux.RLock()
	defer configMux.RUnlock()

	var resolved bool

	if node := resolver.DefaultHosts.Search(metadata.Host); node != nil {
		ip := node.Data.(net.IP)
		metadata.DstIP = ip
		resolved = true
	}

	for _, rule := range rules {
		if !resolved && shouldResolveIP(rule, metadata) {
			ip, err := resolver.ResolveIP(metadata.Host)
			if err != nil {
				log.Debugln("[DNS] resolve %s error: %s", metadata.Host, err.Error())
			} else {
				log.Debugln("[DNS] %s --> %s", metadata.Host, ip.String())
				metadata.DstIP = ip
			}
			resolved = true
		}

		if rule.Match(metadata) {
			adapter, ok := proxies[rule.Adapter()]
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

	return proxies["DIRECT"], nil, nil
}
