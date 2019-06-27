package executor

import (
	"github.com/Dreamacro/clash/component/auth"
	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"
	P "github.com/Dreamacro/clash/proxy"
	authStore "github.com/Dreamacro/clash/proxy/auth"
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
	updateUsers(cfg.Users)
	if force {
		updateGeneral(cfg.General)
	}
	updateProxies(cfg.Proxies)
	updateRules(cfg.Rules)
	updateDNS(cfg.DNS)
	updateExperimental(cfg.Experimental)
}

func GetGeneral() *config.General {
	ports := P.GetPorts()
	return &config.General{
		Port:           ports.Port,
		SocksPort:      ports.SocksPort,
		RedirPort:      ports.RedirPort,
		Authentication: authStore.Authenticator().Users(),
		AllowLan:       P.AllowLan(),
		Mode:           T.Instance().Mode(),
		LogLevel:       log.Level(),
	}
}

func updateExperimental(c *config.Experimental) {
	T.Instance().UpdateExperimental(c.IgnoreResolveFail)
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
		Pool:         c.FakeIPRange,
	})
	T.Instance().SetResolver(r)
	if err := dns.ReCreateServer(c.Listen, r); err != nil {
		log.Errorln("Start DNS server error: %s", err.Error())
		return
	}
	log.Infoln("DNS server listening at: %s", c.Listen)
}

func updateProxies(proxies map[string]C.Proxy) {
	tunnel := T.Instance()
	oldProxies := tunnel.Proxies()

	// close proxy group goroutine
	for _, proxy := range oldProxies {
		proxy.Destroy()
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

func updateUsers(users []auth.AuthUser) {
	authenticator := auth.NewAuthenticator(users)
	authStore.SetAuthenticator(authenticator)
	if authenticator != nil {
		log.Infoln("Authentication of local server updated")
	}
}
