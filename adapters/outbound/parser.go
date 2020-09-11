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
		return nil, fmt.Errorf("Missing type")
	}

	var proxy C.ProxyAdapter
	err := fmt.Errorf("Cannot parse")
	switch proxyType {
	case "ss":
		ssOption := &ShadowSocksOption{}
		err = decoder.Decode(mapping, ssOption)
		if err != nil {
			break
		}
		proxy, err = NewShadowSocks(*ssOption)
	case "ssr":
		ssrOption := &ShadowSocksROption{}
		err = decoder.Decode(mapping, ssrOption)
		if err != nil {
			break
		}
		proxy, err = NewShadowSocksR(*ssrOption)
	case "socks5":
		socksOption := &Socks5Option{}
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
		trojanOption := &TrojanOption{}
		err = decoder.Decode(mapping, trojanOption)
		if err != nil {
			break
		}
		proxy, err = NewTrojan(*trojanOption)
	case "direct":
		directOption := &DirectOption{}
		err = decoder.Decode(mapping, directOption)
		if err != nil {
			break
		}
		proxy = NewDirectWithOption(*directOption)
	default:
		return nil, fmt.Errorf("Unsupport proxy type: %s", proxyType)
	}

	if err != nil {
		return nil, err
	}

	return NewProxy(proxy), nil
}
