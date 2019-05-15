package config

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	adapters "github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/common/structure"
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
	AllowLan           bool         `json:"allow-lan"`
	Mode               T.Mode       `json:"mode"`
	LogLevel           log.LogLevel `json:"log-level"`
	ExternalController string       `json:"-"`
	ExternalUI         string       `json:"-"`
	Secret             string       `json:"-"`
}

// DNS config
type DNS struct {
	Enable       bool             `yaml:"enable"`
	IPv6         bool             `yaml:"ipv6"`
	NameServer   []dns.NameServer `yaml:"nameserver"`
	Fallback     []dns.NameServer `yaml:"fallback"`
	Listen       string           `yaml:"listen"`
	EnhancedMode dns.EnhancedMode `yaml:"enhanced-mode"`
	FakeIPRange  *fakeip.Pool
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
	Rules        []C.Rule
	Proxies      map[string]C.Proxy
}

type rawDNS struct {
	Enable       bool             `yaml:"enable"`
	IPv6         bool             `yaml:"ipv6"`
	NameServer   []string         `yaml:"nameserver"`
	Fallback     []string         `yaml:"fallback"`
	Listen       string           `yaml:"listen"`
	EnhancedMode dns.EnhancedMode `yaml:"enhanced-mode"`
	FakeIPRange  string           `yaml:"fake-ip-range"`
}

type rawConfig struct {
	Port               int          `yaml:"port"`
	SocksPort          int          `yaml:"socks-port"`
	RedirPort          int          `yaml:"redir-port"`
	AllowLan           bool         `yaml:"allow-lan"`
	Mode               T.Mode       `yaml:"mode"`
	LogLevel           log.LogLevel `yaml:"log-level"`
	ExternalController string       `yaml:"external-controller"`
	ExternalUI         string       `yaml:"external-ui"`
	Secret             string       `yaml:"secret"`

	DNS          rawDNS                   `yaml:"dns"`
	Experimental Experimental             `yaml:"experimental"`
	Proxy        []map[string]interface{} `yaml:"Proxy"`
	ProxyGroup   []map[string]interface{} `yaml:"Proxy Group"`
	Rule         []string                 `yaml:"Rule"`
}

func readConfig(path string) (*rawConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("Configuration file %s is empty", C.Path.Config())
	}

	// config with some default value
	rawConfig := &rawConfig{
		AllowLan:   false,
		Mode:       T.Rule,
		LogLevel:   log.INFO,
		Rule:       []string{},
		Proxy:      []map[string]interface{}{},
		ProxyGroup: []map[string]interface{}{},
		Experimental: Experimental{
			IgnoreResolveFail: true,
		},
		DNS: rawDNS{
			Enable:      false,
			FakeIPRange: "198.18.0.1/16",
		},
	}
	err = yaml.Unmarshal([]byte(data), &rawConfig)
	return rawConfig, err
}

// Parse config
func Parse(path string) (*Config, error) {
	config := &Config{}

	rawCfg, err := readConfig(path)
	if err != nil {
		return nil, err
	}
	config.Experimental = &rawCfg.Experimental

	general, err := parseGeneral(rawCfg)
	if err != nil {
		return nil, err
	}
	config.General = general

	proxies, err := parseProxies(rawCfg)
	if err != nil {
		return nil, err
	}
	config.Proxies = proxies

	rules, err := parseRules(rawCfg)
	if err != nil {
		return nil, err
	}
	config.Rules = rules

	dnsCfg, err := parseDNS(rawCfg.DNS)
	if err != nil {
		return nil, err
	}
	config.DNS = dnsCfg

	return config, nil
}

func parseGeneral(cfg *rawConfig) (*General, error) {
	port := cfg.Port
	socksPort := cfg.SocksPort
	redirPort := cfg.RedirPort
	allowLan := cfg.AllowLan
	externalController := cfg.ExternalController
	externalUI := cfg.ExternalUI
	secret := cfg.Secret
	mode := cfg.Mode
	logLevel := cfg.LogLevel

	if externalUI != "" {
		if !filepath.IsAbs(externalUI) {
			externalUI = filepath.Join(C.Path.HomeDir(), externalUI)
		}

		if _, err := os.Stat(externalUI); os.IsNotExist(err) {
			return nil, fmt.Errorf("external-ui: %s not exist", externalUI)
		}
	}

	general := &General{
		Port:               port,
		SocksPort:          socksPort,
		RedirPort:          redirPort,
		AllowLan:           allowLan,
		Mode:               mode,
		LogLevel:           logLevel,
		ExternalController: externalController,
		ExternalUI:         externalUI,
		Secret:             secret,
	}
	return general, nil
}

func parseProxies(cfg *rawConfig) (map[string]C.Proxy, error) {
	proxies := make(map[string]C.Proxy)
	proxyList := []string{}
	proxiesConfig := cfg.Proxy
	groupsConfig := cfg.ProxyGroup

	decoder := structure.NewDecoder(structure.Option{TagName: "proxy", WeaklyTypedInput: true})

	proxies["DIRECT"] = adapters.NewProxy(adapters.NewDirect())
	proxies["REJECT"] = adapters.NewProxy(adapters.NewReject())
	proxyList = append(proxyList, "DIRECT", "REJECT")

	// parse proxy
	for idx, mapping := range proxiesConfig {
		proxyType, existType := mapping["type"].(string)
		if !existType {
			return nil, fmt.Errorf("Proxy %d missing type", idx)
		}

		var proxy C.ProxyAdapter
		err := fmt.Errorf("can't parse")
		switch proxyType {
		case "ss":
			ssOption := &adapters.ShadowSocksOption{}
			err = decoder.Decode(mapping, ssOption)
			if err != nil {
				break
			}
			proxy, err = adapters.NewShadowSocks(*ssOption)
		case "socks5":
			socksOption := &adapters.Socks5Option{}
			err = decoder.Decode(mapping, socksOption)
			if err != nil {
				break
			}
			proxy = adapters.NewSocks5(*socksOption)
		case "http":
			httpOption := &adapters.HttpOption{}
			err = decoder.Decode(mapping, httpOption)
			if err != nil {
				break
			}
			proxy = adapters.NewHttp(*httpOption)
		case "vmess":
			vmessOption := &adapters.VmessOption{}
			err = decoder.Decode(mapping, vmessOption)
			if err != nil {
				break
			}
			proxy, err = adapters.NewVmess(*vmessOption)
		default:
			return nil, fmt.Errorf("Unsupport proxy type: %s", proxyType)
		}

		if err != nil {
			return nil, fmt.Errorf("Proxy [%d]: %s", idx, err.Error())
		}

		if _, exist := proxies[proxy.Name()]; exist {
			return nil, fmt.Errorf("Proxy %s is the duplicate name", proxy.Name())
		}
		proxies[proxy.Name()] = adapters.NewProxy(proxy)
		proxyList = append(proxyList, proxy.Name())
	}

	// parse proxy group
	for idx, mapping := range groupsConfig {
		groupType, existType := mapping["type"].(string)
		groupName, existName := mapping["name"].(string)
		if !existType && existName {
			return nil, fmt.Errorf("ProxyGroup %d: missing type or name", idx)
		}

		if _, exist := proxies[groupName]; exist {
			return nil, fmt.Errorf("ProxyGroup %s: the duplicate name", groupName)
		}
		var group C.ProxyAdapter
		ps := []C.Proxy{}

		err := fmt.Errorf("can't parse")
		switch groupType {
		case "url-test":
			urlTestOption := &adapters.URLTestOption{}
			err = decoder.Decode(mapping, urlTestOption)
			if err != nil {
				break
			}

			ps, err = getProxies(proxies, urlTestOption.Proxies)
			if err != nil {
				return nil, fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewURLTest(*urlTestOption, ps)
		case "select":
			selectorOption := &adapters.SelectorOption{}
			err = decoder.Decode(mapping, selectorOption)
			if err != nil {
				break
			}

			ps, err = getProxies(proxies, selectorOption.Proxies)
			if err != nil {
				return nil, fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewSelector(selectorOption.Name, ps)
		case "fallback":
			fallbackOption := &adapters.FallbackOption{}
			err = decoder.Decode(mapping, fallbackOption)
			if err != nil {
				break
			}

			ps, err = getProxies(proxies, fallbackOption.Proxies)
			if err != nil {
				return nil, fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewFallback(*fallbackOption, ps)
		case "load-balance":
			loadBalanceOption := &adapters.LoadBalanceOption{}
			err = decoder.Decode(mapping, loadBalanceOption)
			if err != nil {
				break
			}

			ps, err = getProxies(proxies, loadBalanceOption.Proxies)
			if err != nil {
				return nil, fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewLoadBalance(*loadBalanceOption, ps)
		}
		if err != nil {
			return nil, fmt.Errorf("Proxy %s: %s", groupName, err.Error())
		}
		proxies[groupName] = adapters.NewProxy(group)
		proxyList = append(proxyList, groupName)
	}

	ps := []C.Proxy{}
	for _, v := range proxyList {
		ps = append(ps, proxies[v])
	}

	global, _ := adapters.NewSelector("GLOBAL", ps)
	proxies["GLOBAL"] = adapters.NewProxy(global)
	return proxies, nil
}

func parseRules(cfg *rawConfig) ([]C.Rule, error) {
	rules := []C.Rule{}

	rulesConfig := cfg.Rule
	// parse rules
	for idx, line := range rulesConfig {
		rule := trimArr(strings.Split(line, ","))
		var (
			payload string
			target  string
		)

		switch len(rule) {
		case 2:
			target = rule[1]
		case 3:
			payload = rule[1]
			target = rule[2]
		default:
			return nil, fmt.Errorf("Rules[%d] [%s] error: format invalid", idx, line)
		}

		rule = trimArr(rule)
		var parsed C.Rule
		switch rule[0] {
		case "DOMAIN":
			parsed = R.NewDomain(payload, target)
		case "DOMAIN-SUFFIX":
			parsed = R.NewDomainSuffix(payload, target)
		case "DOMAIN-KEYWORD":
			parsed = R.NewDomainKeyword(payload, target)
		case "GEOIP":
			parsed = R.NewGEOIP(payload, target)
		case "IP-CIDR", "IP-CIDR6":
			if rule := R.NewIPCIDR(payload, target, false); rule != nil {
				parsed = rule
			}
		// deprecated when bump to 1.0
		case "SOURCE-IP-CIDR":
			fallthrough
		case "SRC-IP-CIDR":
			if rule := R.NewIPCIDR(payload, target, true); rule != nil {
				parsed = rule
			}
		case "SRC-PORT":
			if rule := R.NewPort(payload, target, true); rule != nil {
				parsed = rule
			}
		case "DST-PORT":
			if rule := R.NewPort(payload, target, false); rule != nil {
				parsed = rule
			}
		case "MATCH":
			fallthrough
		// deprecated when bump to 1.0
		case "FINAL":
			parsed = R.NewMatch(target)
		}

		if parsed == nil {
			return nil, fmt.Errorf("Rules[%d] [%s] error: payload invalid", idx, line)
		}

		rules = append(rules, parsed)
	}

	return rules, nil
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

func parseDNS(cfg rawDNS) (*DNS, error) {
	if cfg.Enable && len(cfg.NameServer) == 0 {
		return nil, fmt.Errorf("If DNS configuration is turned on, NameServer cannot be empty")
	}

	dnsCfg := &DNS{
		Enable:       cfg.Enable,
		Listen:       cfg.Listen,
		EnhancedMode: cfg.EnhancedMode,
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
		pool, err := fakeip.New(ipnet)
		if err != nil {
			return nil, err
		}

		dnsCfg.FakeIPRange = pool
	}

	return dnsCfg, nil
}
