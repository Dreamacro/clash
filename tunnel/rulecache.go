package tunnel

import (
	"fmt"

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

// NewRuleMatchCacheKey create unique key
func (ruleCache *RuleMatchLruCache) NewRuleMatchCacheKey(metadata C.Metadata) string {
	srcPort := ""
	if len(metadata.SrcPort) > 0 && ruleCache.withSrcPortKey(metadata.SrcPort) {
		srcPort = metadata.SrcPort
	}
	format := "%s-%s-%s-%s-%s-%s-%d-%s"
	return fmt.Sprintf(format, metadata.NetWork.String(), metadata.Type.String(),
		metadata.SrcIP, metadata.DstIP, srcPort, metadata.DstPort,
		metadata.AddrType, metadata.Host)
}

// Put put result into cache
func (ruleCache *RuleMatchLruCache) Put(key string, proxy C.Proxy, rule C.Rule) {
	log.Debugln("[RULE] put into cache, metadata %s, match rule %s %s, proxy %s",
		key, rule.RuleType(), rule.Payload(), proxy.Name())
	value := ruleMatchCacheValue{proxy: proxy, rule: rule}
	ruleCache.cache.Set(key, value)
}

// Get get proxy from cache
func (ruleCache *RuleMatchLruCache) Get(key string) (C.Proxy, C.Rule, bool) {
	elem, exist := ruleCache.cache.Get(key)
	if exist {
		value := elem.(ruleMatchCacheValue)
		log.Debugln("[RULE] hit in cache, metadata %s, match rule %s %s, proxy %s ",
			key, value.rule.RuleType(), value.rule.Payload(), value.proxy.Name())
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
	lru := cache.NewLRUCache(cache.WithSize(1000), cache.WithAge(60*2))
	return &RuleMatchLruCache{
		srcPorts: make(map[string]bool),
		cache:    lru,
	}
}
