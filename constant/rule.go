package constant

// Rule Type
const (
	Domain RuleType = iota
	DomainSuffix
	DomainKeyword
	GEOIP
	IPCIDR
	SourceIPCIDR
	FINAL
)

type RuleType int

func (rt RuleType) String() string {
	switch rt {
	case Domain:
		return "Domain"
	case DomainSuffix:
		return "DomainSuffix"
	case DomainKeyword:
		return "DomainKeyword"
	case GEOIP:
		return "GEOIP"
	case IPCIDR:
		return "IPCIDR"
	case SourceIPCIDR:
		return "SourceIPCIDR"
	case FINAL:
		return "FINAL"
	default:
		return "Unknow"
	}
}

type Rule interface {
	RuleType() RuleType
	IsMatch(metadata *Metadata) bool
	Adapter() string
	Payload() string
}
