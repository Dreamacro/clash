package outbound

import (
	"fmt"

	"github.com/Dreamacro/clash/common/structure"
	C "github.com/Dreamacro/clash/constant"
)

func ParseProxy(mapping map[string]interface{}) (C.Proxy, error) {
	decoder := structure.NewDecoder(structure.Option{TagName: "proxy", WeaklyTypedInput: true})
	proxyType, existType := mapping["type"].(string)
	if !existType {
		return nil, fmt.Errorf("missing type")
	}

	var (
		proxy C.ProxyAdapter
		err   error
	)
	switch proxyType {
	case "ss":
		ssOption := &ShadowSocksOption{
			UDP: true,
		}
		err = decoder.Decode(mapping, ssOption)
		if err != nil {
			break
		}
		proxy, err = NewShadowSocks(*ssOption)
	case "ssr":
		ssrOption := &ShadowSocksROption{
			UDP: true,
		}
		err = decoder.Decode(mapping, ssrOption)
		if err != nil {
			break
		}
		proxy, err = NewShadowSocksR(*ssrOption)
	case "socks5":
		socksOption := &Socks5Option{
			UDP: true,
		}
		err = decoder.Decode(mapping, socksOption)
		if err != nil {
			break
		}
		proxy = NewSocks5(*socksOption)
	case "http":
		httpOption := &HttpOption{}
		err = decoder.Decode(mapping, httpOption)
		if err != nil {
			break
		}
		proxy = NewHttp(*httpOption)
	case "vmess":
		vmessOption := &VmessOption{
			UDP: true,
			HTTPOpts: HTTPOptions{
				Method: "GET",
				Path:   []string{"/"},
			},
		}
		err = decoder.Decode(mapping, vmessOption)
		if err != nil {
			break
		}
		proxy, err = NewVmess(*vmessOption)
	case "snell":
		snellOption := &SnellOption{}
		err = decoder.Decode(mapping, snellOption)
		if err != nil {
			break
		}
		proxy, err = NewSnell(*snellOption)
	case "trojan":
		trojanOption := &TrojanOption{
			UDP: true,
		}
		err = decoder.Decode(mapping, trojanOption)
		if err != nil {
			break
		}
		proxy, err = NewTrojan(*trojanOption)
	default:
		return nil, fmt.Errorf("unsupport proxy type: %s", proxyType)
	}

	if err != nil {
		return nil, err
	}

	return NewProxy(proxy), nil
}
