package rules

import (
	"strings"
	"errors"

	C "github.com/Dreamacro/clash/constant"
)

type Network struct {
	network string
	adapter string
}

func (n *Network) RuleType() C.RuleType {
	return C.Network
}

func (n *Network) Match(metadata *C.Metadata) bool {
	 return metadata.NetWork.String() == n.network
}

func (n *Network) Adapter() string {
	return n.adapter
}

func (n *Network) Payload() string {
	return n.network
}

func (n *Network) NoResolveIP() bool {
	return true
}

func NewNetwork(network string, adapter string) (*Network, error) {
	net := strings.ToLower(network)
	if (net != "udp" && net != "tcp") {
		return nil, errors.New("Unknown network type")
	}

	ret := &Network{
		network: net,
		adapter: adapter,
	}

	return ret, nil
}
