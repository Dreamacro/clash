package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	adapters "github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/common/observable"
	"github.com/Dreamacro/clash/common/structure"
	C "github.com/Dreamacro/clash/constant"
	R "github.com/Dreamacro/clash/rules"

	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

var (
	config *Config
	once   sync.Once
)

// General config
type General struct {
	Port      int
	SocksPort int
	RedirPort int
	AllowLan  bool
	Mode      Mode
	LogLevel  C.LogLevel
}

// ProxyConfig is update proxy schema
type ProxyConfig struct {
	Port      *int
	SocksPort *int
	RedirPort *int
	AllowLan  *bool
}

// RawConfig is raw config struct
type RawConfig struct {
	Port               int    `yaml:"port"`
	SocksPort          int    `yaml:"socks-port"`
	RedirPort          int    `yaml:"redir-port"`
	AllowLan           bool   `yaml:"allow-lan"`
	Mode               string `yaml:"mode"`
	LogLevel           string `yaml:"log-level"`
	ExternalController string `yaml:"external-controller"`
	Secret             string `yaml:"secret"`

	Proxy      []map[string]interface{} `yaml:"Proxy"`
	ProxyGroup []map[string]interface{} `yaml:"Proxy Group"`
	Rule       []string                 `yaml:"Rule"`
}

// Config is clash config manager
type Config struct {
	general    *General
	rules      []C.Rule
	proxies    map[string]C.Proxy
	lastUpdate time.Time

	event      chan<- interface{}
	reportCh   chan interface{}
	observable *observable.Observable
}

// Event is event of clash config
type Event struct {
	Type    string
	Payload interface{}
}

// Subscribe config stream
func (c *Config) Subscribe() observable.Subscription {
	sub, _ := c.observable.Subscribe()
	return sub
}

// Report return a channel for collecting report message
func (c *Config) Report() chan<- interface{} {
	return c.reportCh
}

func (c *Config) readConfig() (*RawConfig, error) {
	if _, err := os.Stat(C.Path.Config()); os.IsNotExist(err) {
		return nil, err
	}
	data, err := ioutil.ReadFile(C.Path.Config())
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("Configuration file %s is empty", C.Path.Config())
	}

	// config with some default value
	rawConfig := &RawConfig{
		AllowLan:   false,
		Mode:       Rule.String(),
		LogLevel:   C.INFO.String(),
		Rule:       []string{},
		Proxy:      []map[string]interface{}{},
		ProxyGroup: []map[string]interface{}{},
	}
	err = yaml.Unmarshal([]byte(data), &rawConfig)
	return rawConfig, err
}

// Parse config
func (c *Config) Parse() error {
	cfg, err := c.readConfig()
	if err != nil {
		return err
	}

	if err := c.parseGeneral(cfg); err != nil {
		return err
	}

	if err := c.parseProxies(cfg); err != nil {
		return err
	}

	return c.parseRules(cfg)
}

// Proxies return proxies of clash
func (c *Config) Proxies() map[string]C.Proxy {
	return c.proxies
}

// Rules return rules of clash
func (c *Config) Rules() []C.Rule {
	return c.rules
}

// SetMode change mode of clash
func (c *Config) SetMode(mode Mode) {
	c.general.Mode = mode
	c.event <- &Event{Type: "mode", Payload: mode}
}

// SetLogLevel change log level of clash
func (c *Config) SetLogLevel(level C.LogLevel) {
	c.general.LogLevel = level
	c.event <- &Event{Type: "log-level", Payload: level}
}

// General return clash general config
func (c *Config) General() General {
	return *c.general
}

// UpdateRules is a function for hot reload rules
func (c *Config) UpdateRules() error {
	cfg, err := c.readConfig()
	if err != nil {
		return err
	}

	return c.parseRules(cfg)
}

func (c *Config) parseGeneral(cfg *RawConfig) error {
	port := cfg.Port
	socksPort := cfg.SocksPort
	redirPort := cfg.RedirPort
	allowLan := cfg.AllowLan
	logLevelString := cfg.LogLevel
	modeString := cfg.Mode

	mode, exist := ModeMapping[modeString]
	if !exist {
		return fmt.Errorf("General.mode value invalid")
	}

	logLevel, exist := C.LogLevelMapping[logLevelString]
	if !exist {
		return fmt.Errorf("General.log-level value invalid")
	}

	c.general = &General{
		Port:      port,
		SocksPort: socksPort,
		RedirPort: redirPort,
		AllowLan:  allowLan,
		Mode:      mode,
		LogLevel:  logLevel,
	}

	if restAddr := cfg.ExternalController; restAddr != "" {
		c.event <- &Event{Type: "external-controller", Payload: restAddr}
		c.event <- &Event{Type: "secret", Payload: cfg.Secret}
	}

	c.UpdateGeneral(*c.general)
	return nil
}

// UpdateGeneral dispatch update event
func (c *Config) UpdateGeneral(general General) {
	c.UpdateProxy(ProxyConfig{
		Port:      &general.Port,
		SocksPort: &general.SocksPort,
		RedirPort: &general.RedirPort,
		AllowLan:  &general.AllowLan,
	})
	c.event <- &Event{Type: "mode", Payload: general.Mode}
	c.event <- &Event{Type: "log-level", Payload: general.LogLevel}
}

// UpdateProxy dispatch update proxy event
func (c *Config) UpdateProxy(pc ProxyConfig) {
	if pc.AllowLan != nil {
		c.general.AllowLan = *pc.AllowLan
	}

	c.general.Port = *or(pc.Port, &c.general.Port)
	if c.general.Port != 0 && (pc.AllowLan != nil || pc.Port != nil) {
		c.event <- &Event{Type: "http-addr", Payload: genAddr(c.general.Port, c.general.AllowLan)}
	}

	c.general.SocksPort = *or(pc.SocksPort, &c.general.SocksPort)
	if c.general.SocksPort != 0 && (pc.AllowLan != nil || pc.SocksPort != nil) {
		c.event <- &Event{Type: "socks-addr", Payload: genAddr(c.general.SocksPort, c.general.AllowLan)}
	}

	c.general.RedirPort = *or(pc.RedirPort, &c.general.RedirPort)
	if c.general.RedirPort != 0 && (pc.AllowLan != nil || pc.RedirPort != nil) {
		c.event <- &Event{Type: "redir-addr", Payload: genAddr(c.general.RedirPort, c.general.AllowLan)}
	}
}

func (c *Config) parseProxies(cfg *RawConfig) error {
	proxies := make(map[string]C.Proxy)
	proxiesConfig := cfg.Proxy
	groupsConfig := cfg.ProxyGroup

	decoder := structure.NewDecoder(structure.Option{TagName: "proxy", WeaklyTypedInput: true})

	proxies["DIRECT"] = adapters.NewDirect()
	proxies["REJECT"] = adapters.NewReject()

	// parse proxy
	for idx, mapping := range proxiesConfig {
		proxyType, existType := mapping["type"].(string)
		if !existType {
			return fmt.Errorf("Proxy %d missing type", idx)
		}

		var proxy C.Proxy
		var err error
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
		case "vmess":
			vmessOption := &adapters.VmessOption{}
			err = decoder.Decode(mapping, vmessOption)
			if err != nil {
				break
			}
			proxy, err = adapters.NewVmess(*vmessOption)
		default:
			return fmt.Errorf("Unsupport proxy type: %s", proxyType)
		}

		if err != nil {
			return fmt.Errorf("Proxy [%d]: %s", idx, err.Error())
		}

		if _, exist := proxies[proxy.Name()]; exist {
			return fmt.Errorf("Proxy %s is the duplicate name", proxy.Name())
		}
		proxies[proxy.Name()] = proxy
	}

	// parse proxy group
	for idx, mapping := range groupsConfig {
		groupType, existType := mapping["type"].(string)
		groupName, existName := mapping["name"].(string)
		if !existType && existName {
			return fmt.Errorf("ProxyGroup %d: missing type or name", idx)
		}

		if _, exist := proxies[groupName]; exist {
			return fmt.Errorf("ProxyGroup %s: the duplicate name", groupName)
		}
		var group C.Proxy
		var err error
		switch groupType {
		case "url-test":
			urlTestOption := &adapters.URLTestOption{}
			err = decoder.Decode(mapping, urlTestOption)
			if err != nil {
				break
			}

			ps, err := getProxies(proxies, urlTestOption.Proxies)
			if err != nil {
				return fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewURLTest(*urlTestOption, ps)
		case "select":
			selectorOption := &adapters.SelectorOption{}
			err = decoder.Decode(mapping, selectorOption)
			if err != nil {
				break
			}

			ps, err := getProxies(proxies, selectorOption.Proxies)
			if err != nil {
				return fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewSelector(selectorOption.Name, ps)
		case "fallback":
			fallbackOption := &adapters.FallbackOption{}
			err = decoder.Decode(mapping, fallbackOption)
			if err != nil {
				break
			}

			ps, err := getProxies(proxies, fallbackOption.Proxies)
			if err != nil {
				return fmt.Errorf("ProxyGroup %s: %s", groupName, err.Error())
			}
			group, err = adapters.NewFallback(*fallbackOption, ps)
		}
		if err != nil {
			return fmt.Errorf("Proxy %s: %s", groupName, err.Error())
		}
		proxies[groupName] = group
	}

	var ps []C.Proxy
	for _, v := range proxies {
		ps = append(ps, v)
	}

	proxies["GLOBAL"], _ = adapters.NewSelector("GLOBAL", ps)

	// close old goroutine
	for _, proxy := range c.proxies {
		switch raw := proxy.(type) {
		case *adapters.URLTest:
			raw.Close()
		case *adapters.Fallback:
			raw.Close()
		}
	}
	c.proxies = proxies
	c.event <- &Event{Type: "proxies", Payload: proxies}
	return nil
}

func (c *Config) parseRules(cfg *RawConfig) error {
	rules := []C.Rule{}

	rulesConfig := cfg.Rule
	// parse rules
	for _, line := range rulesConfig {
		rule := strings.Split(line, ",")
		if len(rule) < 3 {
			continue
		}
		rule = trimArr(rule)
		switch rule[0] {
		case "DOMAIN":
			rules = append(rules, R.NewDomain(rule[1], rule[2]))
		case "DOMAIN-SUFFIX":
			rules = append(rules, R.NewDomainSuffix(rule[1], rule[2]))
		case "DOMAIN-KEYWORD":
			rules = append(rules, R.NewDomainKeyword(rule[1], rule[2]))
		case "GEOIP":
			rules = append(rules, R.NewGEOIP(rule[1], rule[2]))
		case "IP-CIDR", "IP-CIDR6":
			rules = append(rules, R.NewIPCIDR(rule[1], rule[2]))
		case "FINAL":
			rules = append(rules, R.NewFinal(rule[2]))
		}
	}

	c.rules = rules
	c.event <- &Event{Type: "rules", Payload: rules}
	return nil
}

func (c *Config) handleResponseMessage() {
	for elm := range c.reportCh {
		event := elm.(*Event)
		switch event.Type {
		case "http-addr":
			if event.Payload.(bool) == false {
				log.Errorf("Listening HTTP proxy at %d error", c.general.Port)
				c.general.Port = 0
			}
		case "socks-addr":
			if event.Payload.(bool) == false {
				log.Errorf("Listening SOCKS proxy at %d error", c.general.SocksPort)
				c.general.SocksPort = 0
			}
		case "redir-addr":
			if event.Payload.(bool) == false {
				log.Errorf("Listening Redir proxy at %d error", c.general.RedirPort)
				c.general.RedirPort = 0
			}
		}
	}
}

func newConfig() *Config {
	event := make(chan interface{})
	reportCh := make(chan interface{})
	config := &Config{
		general:    &General{},
		proxies:    make(map[string]C.Proxy),
		rules:      []C.Rule{},
		lastUpdate: time.Now(),

		event:      event,
		reportCh:   reportCh,
		observable: observable.NewObservable(event),
	}
	go config.handleResponseMessage()
	return config
}

// Instance return singleton instance of Config
func Instance() *Config {
	once.Do(func() {
		config = newConfig()
	})
	return config
}
