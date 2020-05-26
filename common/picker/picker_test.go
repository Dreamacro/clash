package picker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	assert.NotNil(t, number)
	assert.Equal(t, number.(int), 1)
}

func TestPicker_Timeout(t *testing.T) {
	picker, ctx := WithTimeout(context.Background(), time.Millisecond*5)
	picker.Go(sleepAndSend(ctx, 20, 1))

	number := picker.Wait()
	assert.Nil(t, number)
	assert.NotNil(t, picker.Error())
}
