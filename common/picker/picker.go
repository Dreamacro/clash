package picker

import "context"

func SelectFast(ctx context.Context, in <-chan interface{}) <-chan interface{} {
	out := make(chan interface{})
	go func() {
		select {
		case p, open := <-in:
			if open {
				out <- p
			}
		case <-ctx.Done():
		}

		close(out)
		for range in {
		}
	}()

	return out
}
