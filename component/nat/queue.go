package nat

import (
	"sync"

	channels "gopkg.in/eapache/channels.v1"
)

type Queue struct {
	mu      sync.Mutex
	mapping map[string]*channels.InfiniteChannel
}

func NewQueue() *Queue {
	return &Queue{
		mapping: make(map[string]*channels.InfiniteChannel),
	}
}

func (q *Queue) Get(key string) (*channels.InfiniteChannel, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if ch, ok := q.mapping[key]; ok {
		return ch, false
	}
	ch := channels.NewInfiniteChannel()
	q.mapping[key] = ch
	return ch, true
}

func (q *Queue) Del(key string) *channels.InfiniteChannel {
	q.mu.Lock()
	defer q.mu.Unlock()

	if ch, ok := q.mapping[key]; ok {
		delete(q.mapping, key)
		return ch
	}
	return nil
}

func (q *Queue) Add(key string, f func()) {
	go func() {
		f()
		if ch := q.Del(key); ch != nil {
			ch.Close()
		}
	}()
}
