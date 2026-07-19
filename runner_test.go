//go:build linux

package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeLED struct {
	writes    []bool
	failAfter int // fail on the Nth write (1-based); 0 = never fail
}

func (f *fakeLED) Set(led bool) error {
	f.writes = append(f.writes, led)
	if f.failAfter > 0 && len(f.writes) >= f.failAfter {
		return errors.New("boom")
	}
	return nil
}

var testSteps = []Step{
	{On: true, Duration: time.Millisecond},
	{On: false, Duration: time.Millisecond},
}

func TestRunPatternRepeats(t *testing.T) {
	f := &fakeLED{}
	if err := runPattern(context.Background(), f, testSteps, 3); err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, true, false, true, false}
	if !reflect.DeepEqual(f.writes, want) {
		t.Errorf("writes = %v, want %v", f.writes, want)
	}
}

func TestRunPatternInfiniteUntilCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := runPattern(ctx, &fakeLED{}, testSteps, 0)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("cancellation took %v, want prompt return", elapsed)
	}
}

func TestRunPatternWriteError(t *testing.T) {
	f := &fakeLED{failAfter: 3}
	err := runPattern(context.Background(), f, testSteps, 5)
	if err == nil || err.Error() != "boom" {
		t.Errorf("err = %v, want boom", err)
	}
	if len(f.writes) != 3 {
		t.Errorf("got %d writes after failure, want 3 (abort immediately)", len(f.writes))
	}
}
