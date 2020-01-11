package provider

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/Dreamacro/clash/adapters/outbound"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"

	"gopkg.in/yaml.v2"
)

const (
	ReservedName = "default"

	fileMode = 0666
)

// Provider Type
const (
	Proxy ProviderType = iota
	Rule
)

// ProviderType defined
type ProviderType int

func (pt ProviderType) String() string {
	switch pt {
	case Proxy:
		return "Proxy"
	case Rule:
		return "Rule"
	default:
		return "Unknown"
	}
}

// Provider interface
type Provider interface {
	Name() string
	VehicleType() VehicleType
	Type() ProviderType
	Initial() error
	Reload() error
	Destroy() error
}

// ProxyProvider interface
type ProxyProvider interface {
	Provider
	Proxies() []C.Proxy
	HealthCheck()
	Update() error
}

type ProxySchema struct {
	Proxies []map[string]interface{} `yaml:"proxies"`
}

type ProxySetProvider struct {
	name        string
	vehicle     Vehicle
	hash        [16]byte
	proxies     []C.Proxy
	healthCheck *HealthCheck
	ticker      *time.Ticker
	updatedAt   *time.Time
}

func (pp *ProxySetProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":        pp.Name(),
		"type":        pp.Type().String(),
		"vehicleType": pp.VehicleType().String(),
		"proxies":     pp.Proxies(),
		"updatedAt":   pp.updatedAt,
	})
}

func (pp *ProxySetProvider) Name() string {
	return pp.name
}

func (pp *ProxySetProvider) Reload() error {
	return nil
}

func (pp *ProxySetProvider) HealthCheck() {
	pp.healthCheck.check()
}

func (pp *ProxySetProvider) Update() error {
	return pp.pull()
}

func (pp *ProxySetProvider) Destroy() error {
	pp.healthCheck.close()

	if pp.ticker != nil {
		pp.ticker.Stop()
	}

	return nil
}

func (pp *ProxySetProvider) Initial() error {
	var buf []byte
	var err error
	if stat, err := os.Stat(pp.vehicle.Path()); err == nil {
		buf, err = ioutil.ReadFile(pp.vehicle.Path())
		modTime := stat.ModTime()
		pp.updatedAt = &modTime
	} else {
		buf, err = pp.vehicle.Read()
	}

	if err != nil {
		return err
	}

	proxies, err := pp.parse(buf)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(pp.vehicle.Path(), buf, fileMode); err != nil {
		return err
	}

	pp.hash = md5.Sum(buf)
	pp.setProxies(proxies)

	// pull proxies automatically
	if pp.ticker != nil {
		go pp.pullLoop()
	}

	return nil
}

func (pp *ProxySetProvider) VehicleType() VehicleType {
	return pp.vehicle.Type()
}

func (pp *ProxySetProvider) Type() ProviderType {
	return Proxy
}

func (pp *ProxySetProvider) Proxies() []C.Proxy {
	return pp.proxies
}

func (pp *ProxySetProvider) pullLoop() {
	for range pp.ticker.C {
		if err := pp.pull(); err != nil {
			log.Warnln("[Provider] %s pull error: %s", pp.Name(), err.Error())
		}
	}
}

func (pp *ProxySetProvider) pull() error {
	buf, err := pp.vehicle.Read()
	if err != nil {
		return err
	}

	now := time.Now()
	hash := md5.Sum(buf)
	if bytes.Equal(pp.hash[:], hash[:]) {
		log.Debugln("[Provider] %s's proxies doesn't change", pp.Name())
		pp.updatedAt = &now
		return nil
	}

	proxies, err := pp.parse(buf)
	if err != nil {
		return err
	}
	log.Infoln("[Provider] %s's proxies update", pp.Name())

	if err := ioutil.WriteFile(pp.vehicle.Path(), buf, fileMode); err != nil {
		return err
	}

	pp.updatedAt = &now
	pp.hash = hash
	pp.setProxies(proxies)

	return nil
}

func (pp *ProxySetProvider) parse(buf []byte) ([]C.Proxy, error) {
	schema := &ProxySchema{}

	if err := yaml.Unmarshal(buf, schema); err != nil {
		return nil, err
	}

	if schema.Proxies == nil {
		return nil, errors.New("File must have a `proxies` field")
	}

	proxies := []C.Proxy{}
	for idx, mapping := range schema.Proxies {
		proxy, err := outbound.ParseProxy(mapping)
		if err != nil {
			return nil, fmt.Errorf("Proxy %d error: %w", idx, err)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, errors.New("File doesn't have any valid proxy")
	}

	return proxies, nil
}

func (pp *ProxySetProvider) setProxies(proxies []C.Proxy) {
	pp.proxies = proxies
	pp.healthCheck.setProxy(proxies)
	go pp.healthCheck.check()
}

func NewProxySetProvider(name string, interval time.Duration, vehicle Vehicle, hc *HealthCheck) *ProxySetProvider {
	var ticker *time.Ticker
	if interval != 0 {
		ticker = time.NewTicker(interval)
	}

	if hc.auto() {
		go hc.process()
	}

	return &ProxySetProvider{
		name:        name,
		vehicle:     vehicle,
		proxies:     []C.Proxy{},
		healthCheck: hc,
		ticker:      ticker,
	}
}

type CompatibleProvider struct {
	name        string
	healthCheck *HealthCheck
	proxies     []C.Proxy
}

func (cp *CompatibleProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":        cp.Name(),
		"type":        cp.Type().String(),
		"vehicleType": cp.VehicleType().String(),
		"proxies":     cp.Proxies(),
	})
}

func (cp *CompatibleProvider) Name() string {
	return cp.name
}

func (cp *CompatibleProvider) Reload() error {
	return nil
}

func (cp *CompatibleProvider) Destroy() error {
	cp.healthCheck.close()
	return nil
}

func (cp *CompatibleProvider) HealthCheck() {
	cp.healthCheck.check()
}

func (cp *CompatibleProvider) Update() error {
	return nil
}

func (cp *CompatibleProvider) Initial() error {
	return nil
}

func (cp *CompatibleProvider) VehicleType() VehicleType {
	return Compatible
}

func (cp *CompatibleProvider) Type() ProviderType {
	return Proxy
}

func (cp *CompatibleProvider) Proxies() []C.Proxy {
	return cp.proxies
}

func NewCompatibleProvider(name string, proxies []C.Proxy, hc *HealthCheck) (*CompatibleProvider, error) {
	if len(proxies) == 0 {
		return nil, errors.New("Provider need one proxy at least")
	}

	if hc.auto() {
		go hc.process()
	}

	return &CompatibleProvider{
		name:        name,
		proxies:     proxies,
		healthCheck: hc,
	}, nil
}
