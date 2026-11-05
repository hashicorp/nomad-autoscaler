// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// stop=true halts retrying; err is optional context on either return value.
type retryFunc func(ctx context.Context) (stop bool, err error)

// retry calls f repeatedly until:
//   - f returns stop=true (success or terminal error)
//   - retryAttempts is exhausted
//   - ctx is cancelled
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
