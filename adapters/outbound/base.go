package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Dreamacro/clash/common/queue"
	C "github.com/Dreamacro/clash/constant"
	"github.com/go-ping/ping"

	"go.uber.org/atomic"
)

type Base struct {
	name           string
	addr           string
	pingAddr       string
	tp             C.AdapterType
	udp            bool
	timeout        int
	maxloss        int
	forbidDuration int
	maxFail        int
	failCount      int
	downFrom       int64
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

func (b *Base) PingAddr() string {
	return b.pingAddr
}

func (b *Base) Timeout() int {
	if b.timeout < 1 {
		return 65535
	} else {
		return b.timeout
	}
}

func (b *Base) MaxLoss() int {
	if b.maxloss < 1 {
		return 101
	} else {
		return b.maxloss
	}
}

func (b *Base) ForbidDuration() int {
	return b.forbidDuration
}

func (b *Base) DownFrom() int64 {
	return b.downFrom
}

func (b *Base) SetDownFrom(t int64) {
	b.downFrom = t
}

func (b *Base) FailCount() int {
	return b.failCount
}

func (b *Base) SetFailCount(t int) {
	b.failCount = t
}

func (b *Base) MaxFail() int {
	return b.maxFail
}

func (b *Base) Forbid() bool {
	return b.forbidDuration > 0 && (time.Now().Unix()-b.downFrom) < int64(b.forbidDuration)
}

func (b *Base) Unwrap(metadata *C.Metadata) C.Proxy {
	return nil
}

func NewBase(name string, addr string, pingAddr string, tp C.AdapterType, udp bool, timeout int, maxloss int, forbidDuration int, maxFail int) *Base {
	return &Base{name, addr, pingAddr, tp, udp, timeout, maxloss, forbidDuration, maxFail, 0, 0}
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
	alive   *atomic.Bool
}

func (p *Proxy) Alive() bool {
	return p.alive.Load() && (!p.Forbid())
}

func (p *Proxy) Dial(metadata *C.Metadata) (C.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	return p.DialContext(ctx, metadata)
}

func (p *Proxy) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	conn, err := p.ProxyAdapter.DialContext(ctx, metadata)
	if err != nil {
		p.SetFailCount(p.FailCount() + 1)
		if p.FailCount() >= p.MaxFail() {
			p.alive.Store(false)
			if p.ForbidDuration() > 0 && p.DownFrom() == 0 {
				p.SetDownFrom(time.Now().Unix())
			}
		}
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
	if !p.alive.Load() {
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

// LastLoss return last history record. if proxy is not alive, return the max value of 100%.
func (p *Proxy) LastLoss() (delay uint16) {
	var min uint16 = 0
	if !p.alive.Load() {
		return min
	}

	last := p.history.Last()
	if last == nil {
		return min
	}
	history := last.(C.DelayHistory)
	if history.Delay == 0 {
		return min
	}
	return history.Loss
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
func (p *Proxy) URLTest(ctx context.Context, url string) (t uint16, l uint16, err error) {
	defer func() {
		if err != nil || t >= uint16(p.Timeout()) || l >= uint16(p.MaxLoss()) {
			p.SetFailCount(p.FailCount() + 1)
			if p.FailCount() >= p.MaxFail() {
				p.alive.Store(false)
				if p.ForbidDuration() > 0 && p.DownFrom() == 0 {
					p.SetDownFrom(time.Now().Unix())
				}
			}
		} else {
			p.alive.Store(true)
			p.SetFailCount(0)

			if !p.Forbid() {
				p.SetDownFrom(0)
			}
		}
		record := C.DelayHistory{Time: time.Now()}
		if err == nil {
			record.Delay = t
			record.Loss = l
			record.DownFrom = p.DownFrom()
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
	//ping check
	host := p.PingAddr()
	if host == "" && p.MaxLoss() > 0 && p.MaxLoss() <= 100 {
		hosts := strings.Split(p.Addr(), ":")
		if len(hosts) == 2 {
			host = hosts[0]
		}
	}
	l = 0
	if host != "" {
		pinger, err2 := ping.NewPinger(host)
		pinger.SetPrivileged(true)
		if err2 != nil {
			return
		}
		pinger.Count = 10
		pinger.Interval = 100 * time.Millisecond
		pinger.Timeout = 2000 * time.Millisecond
		err2 = pinger.Run() // Blocks until finished.
		if err2 != nil {
			return
		}
		stats := pinger.Statistics()
		l = uint16(stats.PacketLoss)
		if l < 100 { //ignore block ping server
			t = t + (l*l/100)*(l*l/100)
		} else {
			l = 0
		}
	}
	return
}

func NewProxy(adapter C.ProxyAdapter) *Proxy {
	return &Proxy{adapter, queue.New(10), atomic.NewBool(true)}
}
