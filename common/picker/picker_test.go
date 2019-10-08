package picker

import (
	"context"
	"testing"
	"time"
)

func sleepAndSend(ctx context.Context, delay int, input interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		timer := time.NewTimer(time.Millisecond * time.Duration(delay))
		select {
		case <-timer.C:
			return input, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func TestPicker_Basic(t *testing.T) {
	picker, ctx := WithContext(context.Background())
	picker.Go(sleepAndSend(ctx, 30, 2))
	picker.Go(sleepAndSend(ctx, 20, 1))

	number := picker.Wait()
	if number != nil && number.(int) != 1 {
		t.Error("should recv 1", number)
	}
}

func TestPicker_Timeout(t *testing.T) {
	picker, ctx, cancel := WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()

	picker.Go(sleepAndSend(ctx, 20, 1))

	number := picker.Wait()
	if number != nil {
		t.Error("should recv nil")
	}
}
