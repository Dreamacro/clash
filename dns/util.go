package dns

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/log"
	yaml "gopkg.in/yaml.v2"

	D "github.com/miekg/dns"
)

var (
	// EnhancedModeMapping is a mapping for EnhancedMode enum
	EnhancedModeMapping = map[string]EnhancedMode{
		FAKEIP.String():  FAKEIP,
		MAPPING.String(): MAPPING,
	}
)

const (
	FAKEIP EnhancedMode = iota
	MAPPING
)

type EnhancedMode int

// UnmarshalYAML unserialize EnhancedMode with yaml
func (e *EnhancedMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tp string
	if err := unmarshal(&tp); err != nil {
		return err
	}
	mode, exist := EnhancedModeMapping[tp]
	if !exist {
		return errors.New("invalid mode")
	}
	*e = mode
	return nil
}

// MarshalYAML serialize EnhancedMode with yaml
func (e EnhancedMode) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(e.String())
}

// UnmarshalJSON unserialize EnhancedMode with json
func (e *EnhancedMode) UnmarshalJSON(data []byte) error {
	var tp string
	json.Unmarshal(data, &tp)
	mode, exist := EnhancedModeMapping[tp]
	if !exist {
		return errors.New("invalid mode")
	}
	*e = mode
	return nil
}

// MarshalJSON serialize EnhancedMode with json
func (e EnhancedMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func (e EnhancedMode) String() string {
	switch e {
	case FAKEIP:
		return "fakeip"
	case MAPPING:
		return "redir-host"
	default:
		return "unknown"
	}
}

func putMsgToCache(c *cache.Cache, key string, msg *D.Msg) {
	if len(msg.Answer) == 0 {
		log.Debugln("[DNS] answer length is zero: %#v", msg)
		return
	}

	ttl := time.Duration(msg.Answer[0].Header().Ttl) * time.Second
	c.Put(key, msg, ttl)
}
