package picker

import (
	"context"
	"testing"
	"time"
)

func sleepAndSend(delay int, in chan<- interface{}, input interface{}) {
	time.Sleep(time.Millisecond * time.Duration(delay))
	in <- input
}

func sleepAndClose(delay int, in chan interface{}) {
	time.Sleep(time.Millisecond * time.Duration(delay))
	close(in)
}

func TestPicker_Basic(t *testing.T) {
	in := make(chan interface{})
	fast := SelectFast(context.Background(), in)
	go sleepAndSend(20, in, 1)
	go sleepAndSend(30, in, 2)
	go sleepAndClose(40, in)

	number, exist := <-fast
	if !exist || number != 1 {
		t.Error("should recv 1", exist, number)
	}
}

func TestPicker_Timeout(t *testing.T) {
	in := make(chan interface{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()
	fast := SelectFast(ctx, in)
	go sleepAndSend(20, in, 1)
	go sleepAndClose(30, in)

	_, exist := <-fast
	if exist {
		t.Error("should recv false")
	}
}
