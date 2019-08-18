package nat

import (
	"sync"
)

type Pool struct {
	mu      sync.Mutex
	mapping map[string]sync.WaitGroup
}

func NewPool() *Pool {
	return &Pool{
		mapping: make(map[string]sync.WaitGroup),
	}
}

func (p *Pool) Get(key string) (sync.WaitGroup, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if wg, ok := p.mapping[key]; ok {
		return wg, false
	} else {
		wg = sync.WaitGroup{}
		wg.Add(1)
		p.mapping[key] = wg
		return wg, true
	}
}

func (p *Pool) Del(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.mapping, key)
}
