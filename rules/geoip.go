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
	country  string
	adapter  string
	isNeedIP bool
}

var defaultGEOIP = GEOIP{
	isNeedIP: true,
}

type GEOIPOption func(*GEOIP)

func WithGEOIPIsNeedIP(b bool) GEOIPOption {
	return func(g *GEOIP) {
		g.isNeedIP = b
	}
}

func (g *GEOIP) RuleType() C.RuleType {
	return C.GEOIP
}

func (g *GEOIP) IsMatch(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if !g.isNeedIP {
		ip = HostToIP(metadata.Host)
	}
	if ip == nil {
		return false
	}
	record, _ := mmdb.Country(*ip)
	return record.Country.IsoCode == g.country
}

func (g *GEOIP) Adapter() string {
	return g.adapter
}

func (g *GEOIP) Payload() string {
	return g.country
}

func (g *GEOIP) IsNeedIP() bool {
	return g.isNeedIP
}

func NewGEOIP(country string, adapter string, opts ...GEOIPOption) *GEOIP {
	once.Do(func() {
		var err error
		mmdb, err = geoip2.Open(C.Path.MMDB())
		if err != nil {
			log.Fatalf("Can't load mmdb: %s", err.Error())
		}
	})
	geoip := defaultGEOIP
	for _, o := range opts {
		o(&geoip)
	}
	geoip.country = country
	geoip.adapter = adapter
	return &geoip
}
