package gun

import (
	"context"
	"sync"
)

type Factory = func(ctx context.Context) (*Trunk, error)

type Pool struct {
	lock    sync.Mutex
	trunk   *Trunk
	factory Factory
}

func (p *Pool) GetTrunk(ctx context.Context) (*Trunk, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.trunk == nil || !p.trunk.client.CanTakeNewRequest() {
		if p.trunk != nil {
			p.trunk.Close()
		}

		trunk, err := p.factory(ctx)
		if err != nil {
			return nil, err
		}

		p.trunk = trunk
	}

	return p.trunk, nil
}

func NewPool(factory Factory) *Pool {
	return &Pool{
		factory: factory,
	}
}
