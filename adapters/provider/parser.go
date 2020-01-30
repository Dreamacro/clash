package provider

import (
	"errors"
	"fmt"
	"time"

	"github.com/Dreamacro/clash/common/structure"
	C "github.com/Dreamacro/clash/constant"
)

var (
	errVehicleType = errors.New("unsupport vehicle type")
)

type healthCheckSchema struct {
	Enable   bool   `provider:"enable"`
	URL      string `provider:"url"`
	Interval int    `provider:"interval"`
}

type proxyProviderSchema struct {
	Type        string            `provider:"type"`
	Path        string            `provider:"path"`
	URL         string            `provider:"url,omitempty"`
	Interval    int               `provider:"interval,omitempty"`
	HealthCheck healthCheckSchema `provider:"health-check,omitempty"`
}

func ParseProxyProvider(name string, mapping map[string]interface{}) (ProxyProvider, error) {
	decoder := structure.NewDecoder(structure.Option{TagName: "provider", WeaklyTypedInput: true})

	schema := &proxyProviderSchema{}
	if err := decoder.Decode(mapping, schema); err != nil {
		return nil, err
	}

	var hcInterval uint = 0
	if schema.HealthCheck.Enable {
		hcInterval = uint(schema.HealthCheck.Interval)
	}
	hc := NewHealthCheck([]C.Proxy{}, schema.HealthCheck.URL, hcInterval)

	path := C.Path.Resolve(schema.Path)

	var vehicle Vehicle
	switch schema.Type {
	case "file":
		vehicle = NewFileVehicle(path)
	case "http":
		vehicle = NewHTTPVehicle(schema.URL, path)
	default:
		return nil, fmt.Errorf("%w: %s", errVehicleType, schema.Type)
	}

	interval := time.Duration(uint(schema.Interval)) * time.Second
	return NewProxySetProvider(name, interval, vehicle, hc), nil
}
