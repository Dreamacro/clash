package dns

import (
	"time"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/log"

	D "github.com/miekg/dns"
)

func putMsgToCache(c *cache.Cache, key string, msg *D.Msg) {
	if len(msg.Answer) == 0 {
		log.Debugln("[DNS] answer length is zero: %#v", msg)
		return
	}

	ttl := time.Duration(msg.Answer[0].Header().Ttl) * time.Second
	c.Put(key, msg, ttl)
}
