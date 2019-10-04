package executor

import (
	"context"

	"github.com/Dreamacro/clash/component/auth"
	trie "github.com/Dreamacro/clash/component/domain-trie"
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
func ApplyConfig(ctx context.Context, cfg *config.Config, force bool) {
	updateUsers(cfg.Users)
	if force {
		updateGeneral(ctx, cfg.General)
	}
	updateProxies(ctx, cfg.Proxies)
	updateRules(cfg.Rules)
	updateDNS(ctx, cfg.DNS)
	updateHosts(cfg.Hosts)
	updateExperimental(cfg.Experimental)
}

func GetGeneral() *config.General {
	ports := P.GetPorts()
	authenticator := []string{}
	if auth := authStore.Authenticator(); auth != nil {
		authenticator = auth.Users()
	}

	general := &config.General{
		Port:           ports.Port,
		SocksPort:      ports.SocksPort,
		RedirPort:      ports.RedirPort,
		Authentication: authenticator,
		AllowLan:       P.AllowLan(),
		BindAddress:    P.BindAddress(),
		Mode:           T.Instance().Mode(),
		LogLevel:       log.Level(),
	}

	return general
}

func updateExperimental(c *config.Experimental) {
	T.Instance().UpdateExperimental(c.IgnoreResolveFail)
}

func updateDNS(ctx context.Context, c *config.DNS) {
	if c.Enable == false {
		dns.DefaultResolver = nil
		dns.ReCreateServer(ctx, "", nil)
		return
	}
	r := dns.New(dns.Config{
		Main:         c.NameServer,
		Fallback:     c.Fallback,
		IPv6:         c.IPv6,
		EnhancedMode: c.EnhancedMode,
		Pool:         c.FakeIPRange,
		FallbackFilter: dns.FallbackFilter{
			GeoIP:  c.FallbackFilter.GeoIP,
			IPCIDR: c.FallbackFilter.IPCIDR,
		},
	})
	dns.DefaultResolver = r
	if err := dns.ReCreateServer(ctx, c.Listen, r); err != nil {
		log.Errorln("Start DNS server error: %s", err.Error())
		return
	}

	if c.Listen != "" {
		log.Infoln("DNS server listening at: %s", c.Listen)
	}
}

func updateHosts(tree *trie.Trie) {
	dns.DefaultHosts = tree
}

func shutdownProxies() {
	tunnel := T.Instance()
	oldProxies := tunnel.Proxies()
	// close proxy group goroutine
	for _, proxy := range oldProxies {
		proxy.Destroy()
	}
}

func updateProxies(ctx context.Context, proxies map[string]C.Proxy) {
	shutdownProxies()
	tunnel := T.Instance()
	tunnel.UpdateProxies(proxies)
	go func() {
		select {
		case <-ctx.Done():
			shutdownProxies()
		}
	}()
}

func updateRules(rules []C.Rule) {
	T.Instance().UpdateRules(rules)
}

func updateGeneral(ctx context.Context, general *config.General) {
	log.SetLevel(general.LogLevel)
	T.Instance().SetMode(general.Mode)

	allowLan := general.AllowLan
	P.SetAllowLan(allowLan)

	bindAddress := general.BindAddress
	P.SetBindAddress(bindAddress)

	if err := P.ReCreateHTTP(general.Port); err != nil {
		log.Errorln("Start HTTP server error: %s", err.Error())
	}

	if err := P.ReCreateSocks(ctx, general.SocksPort); err != nil {
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
