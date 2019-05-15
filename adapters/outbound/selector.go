package adapters

import (
	"encoding/json"
	"errors"
	"net"

	C "github.com/Dreamacro/clash/constant"
)

type Selector struct {
	*Base
	selected  C.Proxy
	proxies   map[string]C.Proxy
	proxyList []string
}

type SelectorOption struct {
	Name    string   `proxy:"name"`
	Proxies []string `proxy:"proxies"`
}

func (s *Selector) Dial(metadata *C.Metadata) (net.Conn, error) {
	return s.selected.Dial(metadata)
}

func (s *Selector) DialUDP(metadata *C.Metadata) (net.PacketConn, net.Addr, error) {
	return s.selected.DialUDP(metadata)
}

func (s *Selector) SupportUDP() bool {
	return s.selected.SupportUDP()
}

func (s *Selector) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": s.Type().String(),
		"now":  s.Now(),
		"all":  s.proxyList,
	})
}

func (s *Selector) Now() string {
	return s.selected.Name()
}

func (s *Selector) Set(name string) error {
	proxy, exist := s.proxies[name]
	if !exist {
		return errors.New("Proxy does not exist")
	}
	s.selected = proxy
	return nil
}

func NewSelector(name string, proxies []C.Proxy) (*Selector, error) {
	if len(proxies) == 0 {
		return nil, errors.New("Provide at least one proxy")
	}

	mapping := make(map[string]C.Proxy)
	proxyList := make([]string, len(proxies))
	for idx, proxy := range proxies {
		mapping[proxy.Name()] = proxy
		proxyList[idx] = proxy.Name()
	}

	s := &Selector{
		Base: &Base{
			name: name,
			tp:   C.Selector,
		},
		proxies:   mapping,
		selected:  proxies[0],
		proxyList: proxyList,
	}
	return s, nil
}
