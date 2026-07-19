//go:build linux

package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// Step is one compiled unit of a pattern: hold the LED on or off for Duration.
type Step struct {
	On       bool
	Duration time.Duration
}

const (
	durShort  = 200 * time.Millisecond
	durMedium = 500 * time.Millisecond
	durLong   = 1000 * time.Millisecond
)

// builtins are the named patterns, themselves written in the template syntax.
var builtins = map[string]string{
	"slow":     "(lo)(lb)",     // slow continuous blink
	"fast":     "(sf)",         // fast continuous flashing
	"twoshort": "(sf)(sf)(lb)", // two short flashes, long pause, repeat
}

// ParseError reports where in the template a pattern is malformed.
type ParseError struct {
	Pos int // byte offset into the template
	Msg string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("position %d: %s", e.Pos, e.Msg)
}

// parsePattern compiles a template like "(sf)(sf)(lb)" into steps. Tokens are
// two-letter codes in parens — duration s|m|l, kind b(reak)|o(n)|f(lash) —
// optionally separated by whitespace.
func parsePattern(s string) ([]Step, error) {
	var steps []Step
	i := 0
	for i < len(s) {
		switch c := s[i]; c {
		case ' ', '\t':
			i++
		case '(':
			end := strings.IndexByte(s[i:], ')')
			if end < 0 {
				return nil, &ParseError{Pos: i, Msg: `unclosed "("`}
			}
			tok, err := stepsForCode(s[i+1:i+end], i)
			if err != nil {
				return nil, err
			}
			steps = append(steps, tok...)
			i += end + 1
		default:
			return nil, &ParseError{Pos: i, Msg: fmt.Sprintf(`expected "(", got %q`, string(c))}
		}
	}
	if len(steps) == 0 {
		return nil, &ParseError{Pos: 0, Msg: "empty pattern"}
	}
	return mergeSteps(steps), nil
}

// stepsForCode translates one token code; pos is the offset of its "(" and is
// only used for error reporting.
func stepsForCode(code string, pos int) ([]Step, error) {
	bad := &ParseError{Pos: pos, Msg: fmt.Sprintf("unknown token %q", code)}
	if len(code) != 2 {
		return nil, bad
	}
	var dur time.Duration
	switch code[0] {
	case 's':
		dur = durShort
	case 'm':
		dur = durMedium
	case 'l':
		dur = durLong
	default:
		return nil, bad
	}
	switch code[1] {
	case 'b':
		return []Step{{On: false, Duration: dur}}, nil
	case 'o':
		return []Step{{On: true, Duration: dur}}, nil
	case 'f':
		// A flash carries a trailing off-gap so adjacent flashes stay
		// visually distinct: (xf) == (xo)(sb).
		return []Step{{On: true, Duration: dur}, {On: false, Duration: durShort}}, nil
	default:
		return nil, bad
	}
}

// mergeSteps coalesces adjacent steps with the same LED state, so raw-on
// tokens concatenate into one longer on-period.
func mergeSteps(steps []Step) []Step {
	var out []Step
	for _, st := range steps {
		if n := len(out); n > 0 && out[n-1].On == st.On {
			out[n-1].Duration += st.Duration
			continue
		}
		out = append(out, st)
	}
	return out
}

// resolvePattern accepts a built-in pattern name or a raw template.
func resolvePattern(arg string) ([]Step, error) {
	if tpl, ok := builtins[arg]; ok {
		return parsePattern(tpl)
	}
	steps, err := parsePattern(arg)
	if err != nil {
		names := slices.Sorted(maps.Keys(builtins))
		return nil, fmt.Errorf("invalid pattern %q: %w (built-in patterns: %s)",
			arg, err, strings.Join(names, ", "))
	}
	return steps, nil
}
