package tunnel

import (
	"github.com/Dreamacro/clash/common/cache"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

var (
	ruleMatchCache *RuleMatchLruCache
)

// RuleMatchLruCache lru cache for rule match result
// Source port always change when creating a new connection.
// If source port is not defined in rule, map key can ommit it, and just set SrcPort null
// Otherwise map key must contain source port
type RuleMatchLruCache struct {
	srcPorts map[string]bool // key is src port
	cache    *cache.LruCache
}

// ruleMatchCacheKey match lru cache key, change Metadata field to primitive type, so it can be used as map key
type ruleMatchCacheKey struct {
	NetWork  C.NetWork
	Type     C.Type
	SrcIP    string
	DstIP    string
	SrcPort  string
	DstPort  string
	AddrType int
	Host     string
}

// ruleMatchCacheValue match lru cache value
type ruleMatchCacheValue struct {
	proxy C.Proxy
	rule  C.Rule
}

// AddSrcPort add source port defined in the SrcPort type rule
func (ruleCache *RuleMatchLruCache) AddSrcPort(port string) {
	ruleCache.srcPorts[port] = true
}

// withSrcPortKey check if map key must contain source port
func (ruleCache *RuleMatchLruCache) withSrcPortKey(port string) bool {
	_, exist := ruleCache.srcPorts[port]
	return exist
}

func (ruleCache *RuleMatchLruCache) newRuleMatchCacheKey(metadata C.Metadata) ruleMatchCacheKey {
	key := ruleMatchCacheKey{
		NetWork:  metadata.NetWork,
		Type:     metadata.Type,
		DstPort:  metadata.DstPort,
		AddrType: metadata.AddrType,
		Host:     metadata.Host,
	}
	if metadata.SrcIP != nil {
		key.SrcIP = metadata.SrcIP.String()
	}
	if metadata.DstIP != nil {
		key.DstIP = metadata.DstIP.String()
	}
	if len(metadata.SrcPort) > 0 && ruleCache.withSrcPortKey(metadata.SrcPort) {
		key.SrcPort = metadata.SrcPort
	}

	return key
}

// Put put result into cache
func (ruleCache *RuleMatchLruCache) Put(metadata C.Metadata, proxy C.Proxy, rule C.Rule) {
	log.Debugln("[RULE] put into cache metadata host %s, dst ip %s, match rule %s %s, proxy %s",
		metadata.Host, metadata.DstIP, rule.RuleType(), rule.Payload(), proxy.Name())
	key := ruleCache.newRuleMatchCacheKey(metadata)
	value := ruleMatchCacheValue{proxy: proxy, rule: rule}
	ruleCache.cache.Set(key, value)
}

// Get get proxy from cache
func (ruleCache *RuleMatchLruCache) Get(metadata C.Metadata) (C.Proxy, C.Rule, bool) {
	key := ruleCache.newRuleMatchCacheKey(metadata)
	elem, exist := ruleCache.cache.Get(key)
	if exist {
		value := elem.(ruleMatchCacheValue)
		log.Debugln("[RULE] metadata host %s, dst ip %s, match rule %s %s, proxy %s hit in cache",
			metadata.Host, metadata.DstIP, value.rule.RuleType(), value.rule.Payload(),
			value.proxy.Name())
		return value.proxy, value.rule, exist
	}
	return nil, nil, exist
}

// Clear clear all the cache
func (ruleCache *RuleMatchLruCache) Clear(ruleUpdate bool) {
	ruleCache.cache.Clear()
	if ruleUpdate {
		ruleCache.srcPorts = make(map[string]bool)
	}
}

// NewRuleMatchLruCache create new RuleMatchLruCache
func NewRuleMatchLruCache() *RuleMatchLruCache {
	return &RuleMatchLruCache{
		srcPorts: make(map[string]bool),
		cache:    cache.NewLRUCache(cache.WithSize(1000)),
	}
}
