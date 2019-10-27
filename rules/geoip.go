package rules

import (
	"sync"

	C "github.com/Dreamacro/clash/constant"

	"github.com/oschwald/geoip2-golang"
	log "github.com/sirupsen/logrus"
)

var (
	mmdb *geoip2.Reader
	once sync.Once
)

type GEOIP struct {
	country     string
	adapter     string
	noResolveIP bool
}

func (g *GEOIP) RuleType() C.RuleType {
	return C.GEOIP
}

func (g *GEOIP) Match(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if ip == nil {
		return false
	}
	record, _ := mmdb.Country(ip)
	return record.Country.IsoCode == g.country
}

func (g *GEOIP) Adapter() string {
	return g.adapter
}

func (g *GEOIP) Payload() string {
	return g.country
}

func (g *GEOIP) NoResolveIP() bool {
	return g.noResolveIP
}

func NewGEOIP(country string, adapter string, noResolveIP bool) *GEOIP {
	once.Do(func() {
		var err error
		mmdb, err = geoip2.Open(C.Path.MMDB())
		if err != nil {
			log.Fatalf("Can't load mmdb: %s", err.Error())
		}
	})

	geoip := &GEOIP{
		country:     country,
		adapter:     adapter,
		noResolveIP: noResolveIP,
	}

	return geoip
}
