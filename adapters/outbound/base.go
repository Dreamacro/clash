package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Dreamacro/clash/common/queue"
	C "github.com/Dreamacro/clash/constant"
)

var (
	defaultURLTestTimeout = time.Second * 5
)

type Base struct {
	name string
	addr string
	tp   C.AdapterType
	udp  bool
}

func (b *Base) Name() string {
	return b.name
}

func (b *Base) Type() C.AdapterType {
	return b.tp
}

func (b *Base) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	return c, errors.New("no support")
}

func (b *Base) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	return nil, errors.New("no support")
}

func (b *Base) SupportUDP() bool {
	return b.udp
}

func (b *Base) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": b.Type().String(),
	})
}

func (b *Base) Addr() string {
	return b.addr
}

func (b *Base) Unwrap(metadata *C.Metadata) C.Proxy {
	return nil
}

func NewBase(name string, addr string, tp C.AdapterType, udp bool) *Base {
	return &Base{name, addr, tp, udp}
}

type conn struct {
	net.Conn
	chain C.Chain
}

func (c *conn) Chains() C.Chain {
	return c.chain
}

func (c *conn) AppendToChains(a C.ProxyAdapter) {
	c.chain = append(c.chain, a.Name())
}

func NewConn(c net.Conn, a C.ProxyAdapter) C.Conn {
	return &conn{c, []string{a.Name()}}
}

type packetConn struct {
	net.PacketConn
	chain C.Chain
}

func (c *packetConn) Chains() C.Chain {
	return c.chain
}

func (c *packetConn) AppendToChains(a C.ProxyAdapter) {
	c.chain = append(c.chain, a.Name())
}

func newPacketConn(pc net.PacketConn, a C.ProxyAdapter) C.PacketConn {
	return &packetConn{pc, []string{a.Name()}}
}

type Proxy struct {
	C.ProxyAdapter
	history *queue.Queue
	alive   uint32
}

func (p *Proxy) Alive() bool {
	return atomic.LoadUint32(&p.alive) > 0
}

func (p *Proxy) Dial(metadata *C.Metadata) (C.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	return p.DialContext(ctx, metadata)
}

func (p *Proxy) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	conn, err := p.ProxyAdapter.DialContext(ctx, metadata)
	if err != nil {
		atomic.StoreUint32(&p.alive, 0)
	}
	return conn, err
}

func (p *Proxy) DelayHistory() []C.DelayHistory {
	queue := p.history.Copy()
	histories := []C.DelayHistory{}
	for _, item := range queue {
		histories = append(histories, item.(C.DelayHistory))
	}
	return histories
}

// LastDelay return last history record. if proxy is not alive, return the max value of uint16.
func (p *Proxy) LastDelay() (delay uint16) {
	var max uint16 = 0xffff
	if atomic.LoadUint32(&p.alive) == 0 {
		return max
	}

	last := p.history.Last()
	if last == nil {
		return max
	}
	history := last.(C.DelayHistory)
	if history.Delay == 0 {
		return max
	}
	return history.Delay
}

func (p *Proxy) MarshalJSON() ([]byte, error) {
	inner, err := p.ProxyAdapter.MarshalJSON()
	if err != nil {
		return inner, err
	}

	mapping := map[string]interface{}{}
	json.Unmarshal(inner, &mapping)
	mapping["history"] = p.DelayHistory()
	mapping["name"] = p.Name()
	return json.Marshal(mapping)
}

// URLTest get the delay for the specified URL
func (p *Proxy) URLTest(ctx context.Context, url string) (t uint16, err error) {
	defer func() {
		if err == nil {
			atomic.StoreUint32(&p.alive, 1)
		} else {
			atomic.StoreUint32(&p.alive, 0)
		}
		record := C.DelayHistory{Time: time.Now()}
		if err == nil {
			record.Delay = t
		}
		p.history.Put(record)
		if p.history.Len() > 10 {
			p.history.Pop()
		}
	}()

	addr, err := urlToMetadata(url)
	if err != nil {
		return
	}

	start := time.Now()
	instance, err := p.DialContext(ctx, &addr)
	if err != nil {
		return
	}
	defer instance.Close()

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return
	}
	req = req.WithContext(ctx)

	transport := &http.Transport{
		Dial: func(string, string) (net.Conn, error) {
			return instance, nil
		},
		// from http.DefaultTransport
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	t = uint16(time.Since(start) / time.Millisecond)
	return
}

func NewProxy(adapter C.ProxyAdapter) *Proxy {
	return &Proxy{adapter, queue.New(10), 1}
}
