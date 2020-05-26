package picker

import (
	"context"
	"sync"
	"time"
)

// Picker provides synchronization, and Context cancelation
// for groups of goroutines working on subtasks of a common task.
// Inspired by errGroup
type Picker struct {
	ctx    context.Context
	cancel func()

	wg sync.WaitGroup

	once    sync.Once
	errOnce sync.Once
	result  interface{}
	err     error
}

func newPicker(ctx context.Context, cancel func()) *Picker {
	return &Picker{
		ctx:    ctx,
		cancel: cancel,
	}
}

// WithContext returns a new Picker and an associated Context derived from ctx.
// and cancel when first element return.
func WithContext(ctx context.Context) (*Picker, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return newPicker(ctx, cancel), ctx
}

// WithTimeout returns a new Picker and an associated Context derived from ctx with timeout.
func WithTimeout(ctx context.Context, timeout time.Duration) (*Picker, context.Context) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return newPicker(ctx, cancel), ctx
}

// Wait blocks until all function calls from the Go method have returned,
// then returns the first nil error result (if any) from them.
func (p *Picker) Wait() interface{} {
	p.wg.Wait()
	if p.cancel != nil {
		p.cancel()
	}
	return p.result
}

// Error return the first error (if all success return nil)
func (p *Picker) Error() error {
	return p.err
}

// Go calls the given function in a new goroutine.
// The first call to return a nil error cancels the group; its result will be returned by Wait.
func (p *Picker) Go(f func() (interface{}, error)) {
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		if ret, err := f(); err == nil {
			p.once.Do(func() {
				p.result = ret
				if p.cancel != nil {
					p.cancel()
				}
			})
		} else {
			p.errOnce.Do(func() {
				p.err = err
			})
		}
	}()
}
