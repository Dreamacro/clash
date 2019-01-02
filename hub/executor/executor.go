package executor

import (
	adapters "github.com/Dreamacro/clash/adapters/outbound"
	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"
	P "github.com/Dreamacro/clash/proxy"
	T "github.com/Dreamacro/clash/tunnel"
)

// Parse config with default config path
func Parse() (*config.Config, error) {
	return ParseWithPath(C.Path.Config())
}

// ParseWithPath parse config with custom config path
func ParseWithPath(path string) (*config.Config, error) {
	return config.Parse(path)
}

// ApplyConfig dispatch configure to all parts
func ApplyConfig(cfg *config.Config, force bool) {
	if force {
		updateGeneral(cfg.General)
	}
	updateProxies(cfg.Proxies)
	updateRules(cfg.Rules)
	updateDNS(cfg.DNS)
}

func GetGeneral() *config.General {
	ports := P.GetPorts()
	return &config.General{
		Port:      ports.Port,
		SocksPort: ports.SocksPort,
		RedirPort: ports.RedirPort,
		AllowLan:  P.AllowLan(),
		Mode:      T.Instance().Mode(),
		LogLevel:  log.Level(),
	}
}

func updateDNS(c *config.DNS) {
	if c.Enable == false {
		T.Instance().SetResolver(nil)
		dns.ReCreateServer("", nil)
		return
	}
	r := dns.New(dns.Config{
		Main:         c.NameServer,
		Fallback:     c.Fallback,
		IPv6:         c.IPv6,
		EnhancedMode: c.EnhancedMode,
	})
	T.Instance().SetResolver(r)
	if err := dns.ReCreateServer(c.Listen, r); err != nil {
		log.Errorln("Start DNS server error: %s", err.Error())
	}
}

func updateProxies(proxies map[string]C.Proxy) {
	tunnel := T.Instance()
	oldProxies := tunnel.Proxies()

	// close old goroutine
	for _, proxy := range oldProxies {
		switch raw := proxy.(type) {
		case *adapters.URLTest:
			raw.Close()
		case *adapters.Fallback:
			raw.Close()
		}
	}

	tunnel.UpdateProxies(proxies)
}

func updateRules(rules []C.Rule) {
	T.Instance().UpdateRules(rules)
}

func updateGeneral(general *config.General) {
	log.SetLevel(general.LogLevel)
	T.Instance().SetMode(general.Mode)

	allowLan := general.AllowLan

	P.SetAllowLan(allowLan)
	if err := P.ReCreateHTTP(general.Port); err != nil {
		log.Errorln("Start HTTP server error: %s", err.Error())
	}

	if err := P.ReCreateSocks(general.SocksPort); err != nil {
		log.Errorln("Start SOCKS5 server error: %s", err.Error())
	}

	if err := P.ReCreateRedir(general.RedirPort); err != nil {
		log.Errorln("Start Redir server error: %s", err.Error())
	}
}
