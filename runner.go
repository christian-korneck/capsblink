//go:build linux

package main

import (
	"context"
	"time"
)

// ledWriter is the runner's view of an LED; satisfied by *sysfsLED.
type ledWriter interface {
	Set(on bool) error
}

// runPattern plays steps repeats times (0 = forever) until ctx is cancelled.
func runPattern(ctx context.Context, led ledWriter, steps []Step, repeats int) error {
	for i := 0; repeats == 0 || i < repeats; i++ {
		for _, st := range steps {
			if err := led.Set(st.On); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(st.Duration):
			}
		}
	}
	return nil
}
