package dns

import "net"

type fallbackFilter interface {
	Match(net.IP) bool
}

type geoipFilter struct{}

func (gf *geoipFilter) Match(ip net.IP) bool {
	if mmdb == nil {
		return false
	}

	record, _ := mmdb.Country(ip)
	return record.Country.IsoCode == "CN" || record.Country.IsoCode == ""
}

type ipnetFilter struct {
	ipnet *net.IPNet
}

func (inf *ipnetFilter) Match(ip net.IP) bool {
	return inf.ipnet.Contains(ip)
}
