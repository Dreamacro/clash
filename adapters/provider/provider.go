package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/Dreamacro/clash/adapters/outbound"
	C "github.com/Dreamacro/clash/constant"

	"gopkg.in/yaml.v2"
)

const (
	ReservedName = "default"
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
	Update() error
}

// ProxyProvider interface
type ProxyProvider interface {
	Provider
	Proxies() []C.Proxy
	HealthCheck()
}

type ProxySchema struct {
	Proxies []map[string]interface{} `yaml:"proxies"`
}

// for auto gc
type ProxySetProvider struct {
	*proxySetProvider
}

type proxySetProvider struct {
	*fetcher
	proxies     []C.Proxy
	healthCheck *HealthCheck
}

func (pp *proxySetProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":        pp.Name(),
		"type":        pp.Type().String(),
		"vehicleType": pp.VehicleType().String(),
		"proxies":     pp.Proxies(),
		"updatedAt":   pp.updatedAt,
	})
}

func (pp *proxySetProvider) Name() string {
	return pp.name
}

func (pp *proxySetProvider) HealthCheck() {
	pp.healthCheck.check()
}

func (pp *proxySetProvider) Update() error {
	elm, same, err := pp.fetcher.Update()
	if err == nil && !same {
		pp.onUpdate(elm)
	}
	return err
}

func (pp *proxySetProvider) Initial() error {
	elm, err := pp.fetcher.Initial()
	if err != nil {
		return err
	}

	pp.onUpdate(elm)
	return nil
}

func (pp *proxySetProvider) Type() ProviderType {
	return Proxy
}

func (pp *proxySetProvider) Proxies() []C.Proxy {
	return pp.proxies
}

func proxiesParse(buf []byte) (interface{}, error) {
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

func (pp *proxySetProvider) setProxies(proxies []C.Proxy) {
	pp.proxies = proxies
	pp.healthCheck.setProxy(proxies)
	go pp.healthCheck.check()
}

func stopProxyProvider(pd *ProxySetProvider) {
	pd.healthCheck.close()
	pd.fetcher.Destroy()
}

func NewProxySetProvider(name string, interval time.Duration, vehicle Vehicle, hc *HealthCheck) *ProxySetProvider {
	if hc.auto() {
		go hc.process()
	}

	pd := &proxySetProvider{
		proxies:     []C.Proxy{},
		healthCheck: hc,
	}

	onUpdate := func(elm interface{}) {
		ret := elm.([]C.Proxy)
		pd.setProxies(ret)
	}

	fetcher := newFetcher(name, interval, vehicle, proxiesParse, onUpdate)
	pd.fetcher = fetcher

	wrapper := &ProxySetProvider{pd}
	runtime.SetFinalizer(wrapper, stopProxyProvider)
	return wrapper
}

// for auto gc
type CompatibleProvider struct {
	*compatibleProvider
}

type compatibleProvider struct {
	name        string
	healthCheck *HealthCheck
	proxies     []C.Proxy
}

func (cp *compatibleProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":        cp.Name(),
		"type":        cp.Type().String(),
		"vehicleType": cp.VehicleType().String(),
		"proxies":     cp.Proxies(),
	})
}

func (cp *compatibleProvider) Name() string {
	return cp.name
}

func (cp *compatibleProvider) HealthCheck() {
	cp.healthCheck.check()
}

func (cp *compatibleProvider) Update() error {
	return nil
}

func (cp *compatibleProvider) Initial() error {
	return nil
}

func (cp *compatibleProvider) VehicleType() VehicleType {
	return Compatible
}

func (cp *compatibleProvider) Type() ProviderType {
	return Proxy
}

func (cp *compatibleProvider) Proxies() []C.Proxy {
	return cp.proxies
}

func stopCompatibleProvider(pd *CompatibleProvider) {
	pd.healthCheck.close()
}

func NewCompatibleProvider(name string, proxies []C.Proxy, hc *HealthCheck) (*CompatibleProvider, error) {
	if len(proxies) == 0 {
		return nil, errors.New("Provider need one proxy at least")
	}

	if hc.auto() {
		go hc.process()
	}

	pd := &compatibleProvider{
		name:        name,
		proxies:     proxies,
		healthCheck: hc,
	}

	wrapper := &CompatibleProvider{pd}
	runtime.SetFinalizer(wrapper, stopCompatibleProvider)
	return wrapper, nil
}
