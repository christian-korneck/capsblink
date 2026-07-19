//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultSysLeds = "/sys/class/leds"

// Keyboard is one keyboard with a caps lock LED.
type Keyboard struct {
	ID     int    // 1-based, assigned after sorting LED dirs alphabetically
	Name   string // human name from <LEDDir>/device/name, else the dir basename
	LEDDir string // e.g. /sys/class/leds/input1::capslock
}

// discoverKeyboards lists all caps lock LEDs under sysLeds. The *::capslock
// glob filters out unrelated LEDs (kbd_backlight, inputN::numlock, ...).
func discoverKeyboards(sysLeds string) ([]Keyboard, error) {
	matches, err := filepath.Glob(filepath.Join(sysLeds, "*::capslock"))
	if err != nil {
		return nil, fmt.Errorf("cannot scan %s: %w", sysLeds, err)
	}
	// Alphabetical order defines the ID assignment ("input10" sorts before
	// "input2"). Glob already sorts, but the contract is explicit here.
	sort.Strings(matches)
	kbds := make([]Keyboard, 0, len(matches))
	for i, dir := range matches {
		name := filepath.Base(dir)
		if b, err := os.ReadFile(filepath.Join(dir, "device", "name")); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				name = s
			}
		}
		kbds = append(kbds, Keyboard{ID: i + 1, Name: name, LEDDir: dir})
	}
	return kbds, nil
}

// sysfsLED writes on/off values to an LED's brightness file.
type sysfsLED struct {
	brightnessPath string
	onVal, offVal  string
}

// openLED prepares brightness writes for the LED. It returns the brightness
// value found at startup so the caller can restore it on exit, and probes
// writability by re-writing that value so a permission problem surfaces
// before any blinking starts.
func openLED(ledDir string) (led *sysfsLED, original string, err error) {
	onVal := "1"
	if b, err := os.ReadFile(filepath.Join(ledDir, "max_brightness")); err == nil {
		s := strings.TrimSpace(string(b))
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			onVal = s
		}
	}
	b, err := os.ReadFile(filepath.Join(ledDir, "brightness"))
	if err != nil {
		return nil, "", err
	}
	original = strings.TrimSpace(string(b))
	led = &sysfsLED{
		brightnessPath: filepath.Join(ledDir, "brightness"),
		onVal:          onVal,
		offVal:         "0",
	}
	vlogf("on value = %q, original brightness = %q", onVal, original)
	if err := led.write(original); err != nil {
		return nil, "", err
	}
	return led, original, nil
}

func (l *sysfsLED) Set(on bool) error {
	if on {
		return l.write(l.onVal)
	}
	return l.write(l.offVal)
}

func (l *sysfsLED) write(val string) error {
	vlogf("write %s <- %s", l.brightnessPath, val)
	return os.WriteFile(l.brightnessPath, []byte(val), 0o644)
}
