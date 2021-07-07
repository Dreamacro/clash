package rules

import (
	"strconv"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

type Port struct {
	adapter  string
	port     string
	isSource bool
	portList []portReal
}

type portReal struct {
	portStart int
	portEnd   int
}

func (p *Port) RuleType() C.RuleType {
	if p.isSource {
		return C.SrcPort
	}
	return C.DstPort
}

func (p *Port) Match(metadata *C.Metadata) bool {
	if p.isSource {
		return p.matchPortReal(metadata.SrcPort)
	}
	return p.matchPortReal(metadata.DstPort)
}

func (p *Port) Adapter() string {
	return p.adapter
}

func (p *Port) Payload() string {
	return p.port
}

func (p *Port) ShouldResolveIP() bool {
	return false
}

func (p *Port) matchPortReal(portRef string) bool {
	port, err := strconv.Atoi(portRef)
	if err != nil {
		return false
	}

	var rs bool
	for _, pr := range p.portList {
		if pr.portEnd == -1 {
			rs = port == pr.portStart
		} else {
			rs = port >= pr.portStart && port <= pr.portEnd
		}
		if rs {
			return true
		}
	}
	return false
}

func NewPort(port string, adapter string, isSource bool) (*Port, error) {
	//the port format should be like this: "123/136/137-139" or "[123]/[136-139]"
	ports := strings.Split(port, "/")

	var portList []portReal
	for _, p := range ports {
		if p == "" {
			continue
		}

		subPorts := strings.Split(p, "-")
		subPortsLen := len(subPorts)
		if subPortsLen > 2 {
			return nil, errPayload
		}

		portStart, err := strconv.Atoi(strings.Trim(subPorts[0], "[ ]"))
		if err != nil || portStart < 0 || portStart > 65535 {
			return nil, errPayload
		}

		if subPortsLen == 1 {
			portList = append(portList, portReal{portStart, -1})

		} else if subPortsLen == 2 {
			portEnd, err1 := strconv.Atoi(strings.Trim(subPorts[1], "[ ]"))
			if err1 != nil || portEnd < 0 || portEnd > 65535 {
				return nil, errPayload
			}

			shouldReverse := portStart > portEnd
			if shouldReverse {
				portList = append(portList, portReal{portEnd, portStart})
			} else {
				portList = append(portList, portReal{portStart, portEnd})
			}
		}
	}

	if len(portList) == 0 {
		return nil, errPayload
	}

	return &Port{
		adapter:  adapter,
		port:     port,
		isSource: isSource,
		portList: portList,
	}, nil
}
