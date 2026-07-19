//go:build linux

package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func on(d time.Duration) Step  { return Step{On: true, Duration: d} }
func off(d time.Duration) Step { return Step{On: false, Duration: d} }

func TestParsePattern(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Step
	}{
		{"single break", "(sb)", []Step{off(200 * time.Millisecond)}},
		{"raw on", "(lo)", []Step{on(time.Second)}},
		{"flash desugars to on plus gap", "(sf)", []Step{on(200 * time.Millisecond), off(200 * time.Millisecond)}},
		{"medium flash", "(mf)", []Step{on(500 * time.Millisecond), off(200 * time.Millisecond)}},
		{"two short flashes long pause", "(sf)(sf)(lb)", []Step{
			on(200 * time.Millisecond), off(200 * time.Millisecond),
			on(200 * time.Millisecond), off(1200 * time.Millisecond),
		}},
		{"raw ons merge", "(so)(so)", []Step{on(400 * time.Millisecond)}},
		{"breaks merge", "(sb)(mb)", []Step{off(700 * time.Millisecond)}},
		{"whitespace between tokens", " (sf) \t(lb) ", []Step{on(200 * time.Millisecond), off(1200 * time.Millisecond)}},
		{"spec example", "(sf)(sf)(lb)(lf)(lb)", []Step{
			on(200 * time.Millisecond), off(200 * time.Millisecond),
			on(200 * time.Millisecond), off(1200 * time.Millisecond),
			on(1000 * time.Millisecond), off(1200 * time.Millisecond),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePattern(tt.in)
			if err != nil {
				t.Fatalf("parsePattern(%q) error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePattern(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFlashEqualsOnPlusBreak(t *testing.T) {
	for _, dur := range []string{"s", "m", "l"} {
		flash, err := parsePattern("(" + dur + "f)")
		if err != nil {
			t.Fatal(err)
		}
		sugar, err := parsePattern("(" + dur + "o)(sb)")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(flash, sugar) {
			t.Errorf("(%sf) = %v, want same as (%so)(sb) = %v", dur, flash, dur, sugar)
		}
	}
}

func TestParsePatternErrors(t *testing.T) {
	tests := []struct {
		in      string
		wantPos int
		wantSub string
	}{
		{"", 0, "empty pattern"},
		{"   ", 0, "empty pattern"},
		{"sf", 0, `expected "("`},
		{"(sf", 0, "unclosed"},
		{"(xf)", 0, "unknown token"},
		{"(s)", 0, "unknown token"},
		{"()", 0, "unknown token"},
		{"(sx)", 0, "unknown token"},
		{"(sf)x(lb)", 4, `expected "("`},
		{"(sf)(xz)", 4, "unknown token"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := parsePattern(tt.in)
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("parsePattern(%q) error = %v, want *ParseError", tt.in, err)
			}
			if pe.Pos != tt.wantPos {
				t.Errorf("parsePattern(%q) error pos = %d, want %d", tt.in, pe.Pos, tt.wantPos)
			}
			if !strings.Contains(pe.Msg, tt.wantSub) {
				t.Errorf("parsePattern(%q) error msg = %q, want substring %q", tt.in, pe.Msg, tt.wantSub)
			}
		})
	}
}

func TestBuiltinsAllParse(t *testing.T) {
	for name, tpl := range builtins {
		if _, err := parsePattern(tpl); err != nil {
			t.Errorf("built-in %q template %q does not parse: %v", name, tpl, err)
		}
	}
}

func TestResolvePattern(t *testing.T) {
	fromName, err := resolvePattern("twoshort")
	if err != nil {
		t.Fatalf("resolvePattern(twoshort) error: %v", err)
	}
	fromTpl, err := resolvePattern(builtins["twoshort"])
	if err != nil {
		t.Fatalf("resolvePattern(raw template) error: %v", err)
	}
	if !reflect.DeepEqual(fromName, fromTpl) {
		t.Errorf("built-in name and raw template disagree: %v vs %v", fromName, fromTpl)
	}

	_, err = resolvePattern("nope")
	if err == nil {
		t.Fatal("resolvePattern(nope) succeeded, want error")
	}
	for _, name := range []string{"fast", "slow", "twoshort"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not hint at built-in %q", err, name)
		}
	}
}
