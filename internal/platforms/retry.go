package platforms

import (
	"context"
	"time"
)

// Retry runs op, then runs it again after each of delays while it fails with an error transient
// accepts: len(delays)+1 attempts at most. It returns op's last error, and stops waiting, without a
// further attempt, once ctx ends.
func Retry(ctx context.Context, delays []time.Duration, transient func(error) bool, op func() error) error {
	err := op()
	for _, d := range delays {
		if err == nil || !transient(err) || ctx.Err() != nil {
			return err
		}
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
		err = op()
	}
	return err
}
