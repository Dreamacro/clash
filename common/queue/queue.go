package queue

import (
	"sync"
)

// Queue is a simple concurrent safe queue
type Queue struct {
	items []interface{}
	lock  sync.RWMutex
}

// Put add the item to the queue.
func (q *Queue) Put(items ...interface{}) {
	if len(items) == 0 {
		return
	}

	q.lock.Lock()
	q.items = append(q.items, items...)
	q.lock.Unlock()
}

// Pop returns the head of items.
func (q *Queue) Pop() interface{} {
	if len(q.items) == 0 {
		return nil
	}

	q.lock.Lock()
	head := q.items[0]
	q.items = q.items[1:]
	q.lock.Unlock()
	return head
}

// Last returns the last of item.
func (q *Queue) Last() interface{} {
	if len(q.items) == 0 {
		return nil
	}

	q.lock.RLock()
	last := q.items[len(q.items)-1]
	q.lock.RUnlock()
	return last
}

// Copy get the copy of queue.
func (q *Queue) Copy() []interface{} {
	items := []interface{}{}
	q.lock.RLock()
	items = append(items, q.items...)
	q.lock.RUnlock()
	return items
}

// Len returns the number of items in this queue.
func (q *Queue) Len() int64 {
	q.lock.Lock()
	defer q.lock.Unlock()

	return int64(len(q.items))
}

// New is a constructor for a new concurrent safe queue.
func New(hint int64) *Queue {
	return &Queue{
		items: make([]interface{}, 0, hint),
	}
}
