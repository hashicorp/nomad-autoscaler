package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// retryFunc is the function signature for a function which is retryable. The
// stop bool indicates whether or not the retry should be halted indicating a
// terminal error. The error return can accompany either a true or false stop
// return to provide context when needed.
type retryFunc func(ctx context.Context) (stop bool, err error)

// retry will retry the passed function f until any of the following conditions
// are met:
//  - the function returns stop=true and err=nil
//  - the retryAttempts limit is reached
//  - the context is cancelled
func retry(ctx context.Context, retryInterval time.Duration, retryAttempts int, f retryFunc) error {

	var (
		retryCount int
		lastErr    error
	)

	for {
		if ctx.Err() != nil {
			if lastErr != nil {
				return fmt.Errorf("retry failed with %v; last error: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		}

		stop, err := f(ctx)
		if stop {
			return err
		}

		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			lastErr = err
		}

		if err == nil {
			return nil
		}

		retryCount++

		if retryCount == retryAttempts {
			return errors.New("reached retry limit")
		}
		time.Sleep(retryInterval)
	}
}
