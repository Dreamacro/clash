package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/adapters/outboundgroup"
	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/component/auth"
	trie "github.com/Dreamacro/clash/component/domain-trie"
	"github.com/Dreamacro/clash/component/fakeip"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"
	R "github.com/Dreamacro/clash/rules"
	T "github.com/Dreamacro/clash/tunnel"

	yaml "gopkg.in/yaml.v2"
)

// General config
type General struct {
	Port               int          `json:"port"`
	SocksPort          int          `json:"socks-port"`
	RedirPort          int          `json:"redir-port"`
	Authentication     []string     `json:"authentication"`
	AllowLan           bool         `json:"allow-lan"`
	BindAddress        string       `json:"bind-address"`
	Mode               T.Mode       `json:"mode"`
	LogLevel           log.LogLevel `json:"log-level"`
	ExternalController string       `json:"-"`
	ExternalUI         string       `json:"-"`
	Secret             string       `json:"-"`
}

// DNS config
type DNS struct {
	Enable         bool             `yaml:"enable"`
	IPv6           bool             `yaml:"ipv6"`
	NameServer     []dns.NameServer `yaml:"nameserver"`
	Fallback       []dns.NameServer `yaml:"fallback"`
	FallbackFilter FallbackFilter   `yaml:"fallback-filter"`
	Listen         string           `yaml:"listen"`
	EnhancedMode   dns.EnhancedMode `yaml:"enhanced-mode"`
	FakeIPRange    *fakeip.Pool
}

// FallbackFilter config
type FallbackFilter struct {
	GeoIP  bool         `yaml:"geoip"`
	IPCIDR []*net.IPNet `yaml:"ipcidr"`
}

// Experimental config
type Experimental struct {
	IgnoreResolveFail bool `yaml:"ignore-resolve-fail"`
}

// Config is clash config manager
type Config struct {
	General      *General
	DNS          *DNS
	Experimental *Experimental
	Hosts        *trie.Trie
	Rules        []C.Rule
	Users        []auth.AuthUser
	Proxies      map[string]C.Proxy
	Providers    map[string]provider.ProxyProvider
}

type RawDNS struct {
	Enable         bool              `yaml:"enable"`
	IPv6           bool              `yaml:"ipv6"`
	NameServer     []string          `yaml:"nameserver"`
	Fallback       []string          `yaml:"fallback"`
	FallbackFilter RawFallbackFilter `yaml:"fallback-filter"`
	Listen         string            `yaml:"listen"`
	EnhancedMode   dns.EnhancedMode  `yaml:"enhanced-mode"`
	FakeIPRange    string            `yaml:"fake-ip-range"`
	FakeIPFilter   []string          `yaml:"fake-ip-filter"`
}

type RawFallbackFilter struct {
	GeoIP  bool     `yaml:"geoip"`
	IPCIDR []string `yaml:"ipcidr"`
}

type RawConfig struct {
	Port               int          `yaml:"port"`
	SocksPort          int          `yaml:"socks-port"`
	RedirPort          int          `yaml:"redir-port"`
	Authentication     []string     `yaml:"authentication"`
	AllowLan           bool         `yaml:"allow-lan"`
	BindAddress        string       `yaml:"bind-address"`
	Mode               T.Mode       `yaml:"mode"`
	LogLevel           log.LogLevel `yaml:"log-level"`
	ExternalController string       `yaml:"external-controller"`
	ExternalUI         string       `yaml:"external-ui"`
	Secret             string       `yaml:"secret"`

	ProxyProvider map[string]map[string]interface{} `yaml:"proxy-provider"`
	Hosts         map[string]string                 `yaml:"hosts"`
	DNS           RawDNS                            `yaml:"dns"`
	Experimental  Experimental                      `yaml:"experimental"`
	Proxy         []map[string]interface{}          `yaml:"Proxy"`
	ProxyGroup    []map[string]interface{}          `yaml:"Proxy Group"`
	Rule          []string                          `yaml:"Rule"`
}

// Parse config
func Parse(buf []byte) (*Config, error) {
	rawCfg, err := UnmarshalRawConfig(buf)
	if err != nil {
		return nil, err
	}

	return ParseRawConfig(rawCfg)
}

func UnmarshalRawConfig(buf []byte) (*RawConfig, error) {
	// config with some default value
	rawCfg := &RawConfig{
		AllowLan:       false,
		BindAddress:    "*",
		Mode:           T.Rule,
		Authentication: []string{},
		LogLevel:       log.INFO,
		Hosts:          map[string]string{},
		Rule:           []string{},
		Proxy:          []map[string]interface{}{},
		ProxyGroup:     []map[string]interface{}{},
		Experimental: Experimental{
			IgnoreResolveFail: true,
		},
		DNS: RawDNS{
			Enable:      false,
			FakeIPRange: "198.18.0.1/16",
			FallbackFilter: RawFallbackFilter{
				GeoIP:  true,
				IPCIDR: []string{},
			},
		},
	}

	if err := yaml.Unmarshal(buf, &rawCfg); err != nil {
		return nil, err
	}

	return rawCfg, nil
}

func ParseRawConfig(rawCfg *RawConfig) (*Config, error) {
	config := &Config{}

	config.Experimental = &rawCfg.Experimental

	general, err := parseGeneral(rawCfg)
	if err != nil {
		return nil, err
	}
	config.General = general

	proxies, providers, err := parseProxies(rawCfg)
	if err != nil {
		return nil, err
	}
	config.Proxies = proxies
	config.Providers = providers

	rules, err := parseRules(rawCfg, proxies)
	if err != nil {
		return nil, err
	}
	config.Rules = rules

	dnsCfg, err := parseDNS(rawCfg.DNS)
	if err != nil {
		return nil, err
	}
	config.DNS = dnsCfg

	hosts, err := parseHosts(rawCfg)
	if err != nil {
		return nil, err
	}
	config.Hosts = hosts

	config.Users = parseAuthentication(rawCfg.Authentication)

	return config, nil
}

func parseGeneral(cfg *RawConfig) (*General, error) {
	port := cfg.Port
	socksPort := cfg.SocksPort
	redirPort := cfg.RedirPort
	allowLan := cfg.AllowLan
	bindAddress := cfg.BindAddress
	externalController := cfg.ExternalController
	externalUI := cfg.ExternalUI
	secret := cfg.Secret
	mode := cfg.Mode
	logLevel := cfg.LogLevel

	if externalUI != "" {
		externalUI = C.Path.Resolve(externalUI)

		if _, err := os.Stat(externalUI); os.IsNotExist(err) {
			return nil, fmt.Errorf("external-ui: %s not exist", externalUI)
		}
	}

	general := &General{
		Port:               port,
		SocksPort:          socksPort,
		RedirPort:          redirPort,
		AllowLan:           allowLan,
		BindAddress:        bindAddress,
		Mode:               mode,
		LogLevel:           logLevel,
		ExternalController: externalController,
		ExternalUI:         externalUI,
		Secret:             secret,
	}
	return general, nil
}

func parseProxies(cfg *RawConfig) (proxies map[string]C.Proxy, providersMap map[string]provider.ProxyProvider, err error) {
	proxies = make(map[string]C.Proxy)
	providersMap = make(map[string]provider.ProxyProvider)
	proxyList := []string{}
	proxiesConfig := cfg.Proxy
	groupsConfig := cfg.ProxyGroup
	providersConfig := cfg.ProxyProvider

	defer func() {
		// Destroy already created provider when err != nil
		if err != nil {
			for _, provider := range providersMap {
				provider.Destroy()
			}
		}
	}()

	proxies["DIRECT"] = outbound.NewProxy(outbound.NewDirect())
	proxies["REJECT"] = outbound.NewProxy(outbound.NewReject())
	proxyList = append(proxyList, "DIRECT", "REJECT")

	// parse proxy
	for idx, mapping := range proxiesConfig {
		proxy, err := outbound.ParseProxy(mapping)
		if err != nil {
			return nil, nil, fmt.Errorf("Proxy %d: %w", idx, err)
		}

		if _, exist := proxies[proxy.Name()]; exist {
			return nil, nil, fmt.Errorf("Proxy %s is the duplicate name", proxy.Name())
		}
		proxies[proxy.Name()] = proxy
		proxyList = append(proxyList, proxy.Name())
	}

	// keep the origional order of ProxyGroups in config file
	for idx, mapping := range groupsConfig {
		groupName, existName := mapping["name"].(string)
		if !existName {
			return nil, nil, fmt.Errorf("ProxyGroup %d: missing name", idx)
		}
		proxyList = append(proxyList, groupName)
	}

	// check if any loop exists and sort the ProxyGroups
	if err := proxyGroupsDagSort(groupsConfig); err != nil {
		return nil, nil, err
	}

	// parse and initial providers
	for name, mapping := range providersConfig {
		if name == provider.ReservedName {
			return nil, nil, fmt.Errorf("can not defined a provider called `%s`", provider.ReservedName)
		}

		pd, err := provider.ParseProxyProvider(name, mapping)
		if err != nil {
			return nil, nil, err
		}

		providersMap[name] = pd
	}

	for _, provider := range providersMap {
		log.Infoln("Start initial provider %s", provider.Name())
		if err := provider.Initial(); err != nil {
			return nil, nil, err
		}
	}

	// parse proxy group
	for idx, mapping := range groupsConfig {
		group, err := outboundgroup.ParseProxyGroup(mapping, proxies, providersMap)
		if err != nil {
			return nil, nil, fmt.Errorf("ProxyGroup[%d]: %w", idx, err)
		}

		groupName := group.Name()
		if _, exist := proxies[groupName]; exist {
			return nil, nil, fmt.Errorf("ProxyGroup %s: the duplicate name", groupName)
		}

		proxies[groupName] = outbound.NewProxy(group)
	}

	// initial compatible provider
	for _, pd := range providersMap {
		if pd.VehicleType() != provider.Compatible {
			continue
		}

		log.Infoln("Start initial compatible provider %s", pd.Name())
		if err := pd.Initial(); err != nil {
			return nil, nil, err
		}
	}

	ps := []C.Proxy{}
	for _, v := range proxyList {
		ps = append(ps, proxies[v])
	}
	hc := provider.NewHealthCheck(ps, "", 0)
	pd, _ := provider.NewCompatibleProvider(provider.ReservedName, ps, hc)
	providersMap[provider.ReservedName] = pd

	global := outboundgroup.NewSelector("GLOBAL", []provider.ProxyProvider{pd})
	proxies["GLOBAL"] = outbound.NewProxy(global)
	return proxies, providersMap, nil
}

func parseRules(cfg *RawConfig, proxies map[string]C.Proxy) ([]C.Rule, error) {
	rules := []C.Rule{}

	rulesConfig := cfg.Rule
	// parse rules
	for idx, line := range rulesConfig {
		rule := trimArr(strings.Split(line, ","))
		var (
			payload string
			target  string
			params  = []string{}
		)

		switch l := len(rule); {
		case l == 2:
			target = rule[1]
		case l == 3:
			payload = rule[1]
			target = rule[2]
		case l >= 4:
			payload = rule[1]
			target = rule[2]
			params = rule[3:]
		default:
			return nil, fmt.Errorf("Rules[%d] [%s] error: format invalid", idx, line)
		}

		if _, ok := proxies[target]; !ok {
			return nil, fmt.Errorf("Rules[%d] [%s] error: proxy [%s] not found", idx, line, target)
		}

		rule = trimArr(rule)
		params = trimArr(params)
		var (
			parseErr error
			parsed   C.Rule
		)

		switch rule[0] {
		case "DOMAIN":
			parsed = R.NewDomain(payload, target)
		case "DOMAIN-SUFFIX":
			parsed = R.NewDomainSuffix(payload, target)
		case "DOMAIN-KEYWORD":
			parsed = R.NewDomainKeyword(payload, target)
		case "GEOIP":
			noResolve := R.HasNoResolve(params)
			parsed = R.NewGEOIP(payload, target, noResolve)
		case "IP-CIDR", "IP-CIDR6":
			noResolve := R.HasNoResolve(params)
			parsed, parseErr = R.NewIPCIDR(payload, target, R.WithIPCIDRNoResolve(noResolve))
		// deprecated when bump to 1.0
		case "SOURCE-IP-CIDR":
			fallthrough
		case "SRC-IP-CIDR":
			parsed, parseErr = R.NewIPCIDR(payload, target, R.WithIPCIDRSourceIP(true), R.WithIPCIDRNoResolve(true))
		case "SRC-PORT":
			parsed, parseErr = R.NewPort(payload, target, true)
		case "DST-PORT":
			parsed, parseErr = R.NewPort(payload, target, false)
		case "MATCH":
			fallthrough
		// deprecated when bump to 1.0
		case "FINAL":
			parsed = R.NewMatch(target)
		default:
			parseErr = fmt.Errorf("unsupported rule type %s", rule[0])
		}

		if parseErr != nil {
			return nil, fmt.Errorf("Rules[%d] [%s] error: %s", idx, line, parseErr.Error())
		}

		rules = append(rules, parsed)
	}

	return rules, nil
}

func parseHosts(cfg *RawConfig) (*trie.Trie, error) {
	tree := trie.New()
	if len(cfg.Hosts) != 0 {
		for domain, ipStr := range cfg.Hosts {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return nil, fmt.Errorf("%s is not a valid IP", ipStr)
			}
			tree.Insert(domain, ip)
		}
	}

	return tree, nil
}

func hostWithDefaultPort(host string, defPort string) (string, error) {
	if !strings.Contains(host, ":") {
		host += ":"
	}

	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		return "", err
	}

	if port == "" {
		port = defPort
	}

	return net.JoinHostPort(hostname, port), nil
}

func parseNameServer(servers []string) ([]dns.NameServer, error) {
	nameservers := []dns.NameServer{}

	for idx, server := range servers {
		// parse without scheme .e.g 8.8.8.8:53
		if !strings.Contains(server, "://") {
			server = "udp://" + server
		}
		u, err := url.Parse(server)
		if err != nil {
			return nil, fmt.Errorf("DNS NameServer[%d] format error: %s", idx, err.Error())
		}

		var host, dnsNetType string
		switch u.Scheme {
		case "udp":
			host, err = hostWithDefaultPort(u.Host, "53")
			dnsNetType = "" // UDP
		case "tcp":
			host, err = hostWithDefaultPort(u.Host, "53")
			dnsNetType = "tcp" // TCP
		case "tls":
			host, err = hostWithDefaultPort(u.Host, "853")
			dnsNetType = "tcp-tls" // DNS over TLS
		case "https":
			clearURL := url.URL{Scheme: "https", Host: u.Host, Path: u.Path}
			host = clearURL.String()
			dnsNetType = "https" // DNS over HTTPS
		default:
			return nil, fmt.Errorf("DNS NameServer[%d] unsupport scheme: %s", idx, u.Scheme)
		}

		if err != nil {
			return nil, fmt.Errorf("DNS NameServer[%d] format error: %s", idx, err.Error())
		}

		nameservers = append(
			nameservers,
			dns.NameServer{
				Net:  dnsNetType,
				Addr: host,
			},
		)
	}
	return nameservers, nil
}

func parseFallbackIPCIDR(ips []string) ([]*net.IPNet, error) {
	ipNets := []*net.IPNet{}

	for idx, ip := range ips {
		_, ipnet, err := net.ParseCIDR(ip)
		if err != nil {
			return nil, fmt.Errorf("DNS FallbackIP[%d] format error: %s", idx, err.Error())
		}
		ipNets = append(ipNets, ipnet)
	}

	return ipNets, nil
}

func parseDNS(cfg RawDNS) (*DNS, error) {
	if cfg.Enable && len(cfg.NameServer) == 0 {
		return nil, fmt.Errorf("If DNS configuration is turned on, NameServer cannot be empty")
	}

	dnsCfg := &DNS{
		Enable:       cfg.Enable,
		Listen:       cfg.Listen,
		IPv6:         cfg.IPv6,
		EnhancedMode: cfg.EnhancedMode,
		FallbackFilter: FallbackFilter{
			IPCIDR: []*net.IPNet{},
		},
	}
	var err error
	if dnsCfg.NameServer, err = parseNameServer(cfg.NameServer); err != nil {
		return nil, err
	}

	if dnsCfg.Fallback, err = parseNameServer(cfg.Fallback); err != nil {
		return nil, err
	}

	if cfg.EnhancedMode == dns.FAKEIP {
		_, ipnet, err := net.ParseCIDR(cfg.FakeIPRange)
		if err != nil {
			return nil, err
		}

		var host *trie.Trie
		// fake ip skip host filter
		if len(cfg.FakeIPFilter) != 0 {
			host = trie.New()
			for _, domain := range cfg.FakeIPFilter {
				host.Insert(domain, true)
			}
		}

		pool, err := fakeip.New(ipnet, 1000, host)
		if err != nil {
			return nil, err
		}

		dnsCfg.FakeIPRange = pool
	}

	dnsCfg.FallbackFilter.GeoIP = cfg.FallbackFilter.GeoIP
	if fallbackip, err := parseFallbackIPCIDR(cfg.FallbackFilter.IPCIDR); err == nil {
		dnsCfg.FallbackFilter.IPCIDR = fallbackip
	}

	return dnsCfg, nil
}

func parseAuthentication(rawRecords []string) []auth.AuthUser {
	users := make([]auth.AuthUser, 0)
	for _, line := range rawRecords {
		userData := strings.SplitN(line, ":", 2)
		if len(userData) == 2 {
			users = append(users, auth.AuthUser{User: userData[0], Pass: userData[1]})
		}
	}
	return users
}
