package singledo

import (
	"sync"
	"time"
)

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Single struct {
	mux    sync.Mutex
	last   time.Time
	wait   time.Duration
	call   *call
	result *Result
}

type Result struct {
	Val interface{}
	Err error
}

// Do single.Do likes sync.singleFlight
//lint:ignore ST1008 it likes sync.singleFlight
func (s *Single) Do(fn func() (interface{}, error)) (v interface{}, err error, shared bool) {
	s.mux.Lock()
	now := time.Now()
	if now.Before(s.last.Add(s.wait)) {
		s.mux.Unlock()
		return s.result.Val, s.result.Err, true
	}

	if call := s.call; call != nil {
		s.mux.Unlock()
		call.wg.Wait()
		return call.val, call.err, true
	}

	call := &call{}
	call.wg.Add(1)
	s.call = call
	s.mux.Unlock()
	call.val, call.err = fn()
	call.wg.Done()

	s.mux.Lock()
	s.call = nil
	s.result = &Result{call.val, call.err}
	s.last = now
	s.mux.Unlock()
	return call.val, call.err, false
}

func (s *Single) Reset() {
	s.last = time.Time{}
}

func NewSingle(wait time.Duration) *Single {
	return &Single{wait: wait}
}
