package adapters

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/Dreamacro/clash/common/queue"
	C "github.com/Dreamacro/clash/constant"
)

type Base struct {
	name string
	tp   C.AdapterType
}

func (b *Base) Name() string {
	return b.name
}

func (b *Base) Type() C.AdapterType {
	return b.tp
}

func (b *Base) Destroy() {}

func (b *Base) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": b.Type().String(),
	})
}

type Proxy struct {
	C.ProxyAdapter
	history *queue.Queue
	alive   bool
}

func (p *Proxy) Alive() bool {
	return p.alive
}

func (p *Proxy) Dial(metadata *C.Metadata) (net.Conn, error) {
	conn, err := p.ProxyAdapter.Dial(metadata)
	p.alive = p.alive && (err == nil)
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

func (p *Proxy) LastDelay() (delay uint16) {
	head := p.history.First()
	if head == nil {
		delay--
		return
	}
	history := head.(C.DelayHistory)
	if history.Delay == 0 {
		delay--
		return
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
	return json.Marshal(mapping)
}

// URLTest get the delay for the specified URL
func (p *Proxy) URLTest(url string) (t uint16, err error) {
	defer func() {
		record := C.DelayHistory{Time: time.Now()}
		if err == nil {
			record.Delay = t
		}
		p.alive = err == nil
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
	instance, err := p.ProxyAdapter.Dial(&addr)
	if err != nil {
		return
	}
	defer instance.Close()
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
	client := http.Client{Transport: transport}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	resp.Body.Close()
	t = uint16(time.Since(start) / time.Millisecond)
	return
}

func NewProxy(adapter C.ProxyAdapter) *Proxy {
	return &Proxy{adapter, queue.New(10), true}
}
