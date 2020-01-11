package outboundgroup

import (
	"errors"
	"fmt"

	"github.com/Dreamacro/clash/adapters/provider"
	"github.com/Dreamacro/clash/common/structure"
	C "github.com/Dreamacro/clash/constant"
)

var (
	errFormat            = errors.New("format error")
	errType              = errors.New("unsupport type")
	errMissUse           = errors.New("`use` field should not be empty")
	errMissProxy         = errors.New("`use` or `proxies` missing")
	errMissHealthCheck   = errors.New("`url` or `interval` missing")
	errDuplicateProvider = errors.New("`duplicate provider name")
)

type GroupCommonOption struct {
	Name     string   `group:"name"`
	Type     string   `group:"type"`
	Proxies  []string `group:"proxies,omitempty"`
	Use      []string `group:"use,omitempty"`
	URL      string   `group:"url,omitempty"`
	Interval int      `group:"interval,omitempty"`
}

func ParseProxyGroup(config map[string]interface{}, proxyMap map[string]C.Proxy, providersMap map[string]provider.ProxyProvider) (C.ProxyAdapter, error) {
	decoder := structure.NewDecoder(structure.Option{TagName: "group", WeaklyTypedInput: true})

	groupOption := &GroupCommonOption{}
	if err := decoder.Decode(config, groupOption); err != nil {
		return nil, errFormat
	}

	if groupOption.Type == "" || groupOption.Name == "" {
		return nil, errFormat
	}

	groupName := groupOption.Name

	providers := []provider.ProxyProvider{}

	if len(groupOption.Proxies) == 0 && len(groupOption.Use) == 0 {
		return nil, errMissProxy
	}

	if len(groupOption.Proxies) != 0 {
		ps, err := getProxies(proxyMap, groupOption.Proxies)
		if err != nil {
			return nil, err
		}

		// if Use not empty, drop health check options
		if len(groupOption.Use) != 0 {
			hc := provider.NewHealthCheck(ps, "", 0)
			pd, err := provider.NewCompatibleProvider(groupName, ps, hc)
			if err != nil {
				return nil, err
			}

			providers = append(providers, pd)
		} else {
			// select don't need health check
			if groupOption.Type == "select" {
				hc := provider.NewHealthCheck(ps, "", 0)
				pd, err := provider.NewCompatibleProvider(groupName, ps, hc)
				if err != nil {
					return nil, err
				}

				providers = append(providers, pd)
				providersMap[groupName] = pd
			} else {
				if groupOption.URL == "" || groupOption.Interval == 0 {
					return nil, errMissHealthCheck
				}

				hc := provider.NewHealthCheck(ps, groupOption.URL, uint(groupOption.Interval))
				pd, err := provider.NewCompatibleProvider(groupName, ps, hc)
				if err != nil {
					return nil, err
				}

				providers = append(providers, pd)
				providersMap[groupName] = pd
			}
		}
	}

	if len(groupOption.Use) != 0 {
		list, err := getProviders(providersMap, groupOption.Use)
		if err != nil {
			return nil, err
		}
		providers = append(providers, list...)
	}

	var group C.ProxyAdapter
	switch groupOption.Type {
	case "url-test":
		group = NewURLTest(groupName, providers)
	case "select":
		group = NewSelector(groupName, providers)
	case "fallback":
		group = NewFallback(groupName, providers)
	case "load-balance":
		group = NewLoadBalance(groupName, providers)
	default:
		return nil, fmt.Errorf("%w: %s", errType, groupOption.Type)
	}

	return group, nil
}

func getProxies(mapping map[string]C.Proxy, list []string) ([]C.Proxy, error) {
	var ps []C.Proxy
	for _, name := range list {
		p, ok := mapping[name]
		if !ok {
			return nil, fmt.Errorf("'%s' not found", name)
		}
		ps = append(ps, p)
	}
	return ps, nil
}

func getProviders(mapping map[string]provider.ProxyProvider, list []string) ([]provider.ProxyProvider, error) {
	var ps []provider.ProxyProvider
	for _, name := range list {
		p, ok := mapping[name]
		if !ok {
			return nil, fmt.Errorf("'%s' not found", name)
		}

		if p.VehicleType() == provider.Compatible {
			return nil, fmt.Errorf("proxy group %s can't contains in `use`", name)
		}
		ps = append(ps, p)
	}
	return ps, nil
}
