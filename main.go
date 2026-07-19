//go:build linux

// Capsblink blinks a keyboard's caps lock LED via /sys/class/leds, driven by
// blink patterns written in a small template syntax.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const usageText = `Usage: capsblink [flags]

Blink the keyboard caps lock LED via /sys/class/leds/inputN::capslock.

  -p, --pattern P   built-in pattern name or raw template (default "slow")
  -n, --number N    number of pattern repeats, 0 = forever (default 0)
  -d, --device ID   keyboard ID as shown by --list (default 1 = first)
  -l, --list        list detected keyboards and exit
  -v, --verbose     debug output (prints sysfs writes)
  -h, --help        show this help

Template tokens, written in parens and concatenated, e.g. "(sf)(sf)(lb)":
  (sb) (mb) (lb)   break:  LED off for short/medium/long
  (sf) (mf) (lf)   flash:  LED on, plus an implicit trailing short off-gap
  (so) (mo) (lo)   raw on: LED on, no gap (adjacent raw-ons merge)
Durations: short=200ms, medium=500ms, long=1s.  (sf) == (so)(sb)

Built-in patterns:
  slow      (lo)(lb)      slow continuous blink
  fast      (sf)          fast continuous flashing
  twoshort  (sf)(sf)(lb)  two short flashes, long pause, repeat
`

// vlog carries -v/--verbose output; it stays discarded otherwise.
var vlog = log.New(io.Discard, "", 0)

func vlogf(format string, args ...any) { vlog.Printf(format, args...) }

func main() {
	os.Exit(cliMain())
}

func cliMain() int {
	var (
		patternArg string
		repeats    int
		deviceID   int
		listOnly   bool
		verbose    bool
	)
	flag.StringVar(&patternArg, "p", "slow", "")
	flag.StringVar(&patternArg, "pattern", "slow", "")
	flag.IntVar(&repeats, "n", 0, "")
	flag.IntVar(&repeats, "number", 0, "")
	flag.IntVar(&deviceID, "d", 1, "")
	flag.IntVar(&deviceID, "device", 1, "")
	flag.BoolVar(&listOnly, "l", false, "")
	flag.BoolVar(&listOnly, "list", false, "")
	flag.BoolVar(&verbose, "v", false, "")
	flag.BoolVar(&verbose, "verbose", false, "")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usageText) }
	flag.Parse()

	if verbose {
		vlog.SetOutput(os.Stderr)
	}
	if repeats < 0 {
		return usageErr("--number must be >= 0")
	}
	if deviceID < 1 {
		return usageErr("--device must be >= 1")
	}
	steps, err := resolvePattern(patternArg)
	if err != nil {
		return usageErr(err.Error())
	}
	vlogf("pattern %q -> %d step(s)", patternArg, len(steps))

	kbds, err := discoverKeyboards(defaultSysLeds)
	if err != nil {
		return fail(err.Error())
	}
	if len(kbds) == 0 {
		return fail("no capslock LEDs found under " + defaultSysLeds)
	}
	vlogf("found %d capslock LED(s) under %s", len(kbds), defaultSysLeds)

	if listOnly {
		for _, k := range kbds {
			if verbose {
				fmt.Printf("%d\t%s\t%s\n", k.ID, k.Name, k.LEDDir)
			} else {
				fmt.Printf("%d\t%s\n", k.ID, k.Name)
			}
		}
		return 0
	}

	if deviceID > len(kbds) {
		return usageErr(fmt.Sprintf("device %d not found: %d keyboard(s) available (run capsblink -l)",
			deviceID, len(kbds)))
	}
	kb := kbds[deviceID-1]
	vlogf("using device %d: %s (%s)", kb.ID, kb.Name, kb.LEDDir)

	led, original, err := openLED(kb.LEDDir)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			fmt.Fprintf(os.Stderr, "capsblink: %v\n", err)
			fmt.Fprintln(os.Stderr, "hint: LED brightness files are usually writable by root only — try: sudo capsblink")
			return 1
		}
		return fail(err.Error())
	}

	// A closed stderr pipe (e.g. `capsblink -v 2>&1 | head`) must not kill
	// the process before the restore below runs; without this the Go
	// runtime forwards EPIPE as a fatal SIGPIPE.
	signal.Ignore(syscall.SIGPIPE)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	// LIFO: the restore below runs before stop(), so a second Ctrl-C during
	// the restore write is still absorbed as an inert cancellation.
	defer func() {
		if err := led.write(original); err != nil {
			fmt.Fprintf(os.Stderr, "capsblink: failed to restore LED state: %v\n", err)
		}
	}()

	if err := runPattern(ctx, led, steps, repeats); err != nil && !errors.Is(err, context.Canceled) {
		return fail(err.Error())
	}
	return 0
}

func usageErr(msg string) int {
	fmt.Fprintf(os.Stderr, "capsblink: %s\n", msg)
	return 2
}

func fail(msg string) int {
	fmt.Fprintf(os.Stderr, "capsblink: %s\n", msg)
	return 1
}
