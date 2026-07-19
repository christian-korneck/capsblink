//go:build linux

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverKeyboards(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"input3::capslock", "input1::capslock", "input1::numlock", "kbd_backlight"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dir, "input1::capslock", "device", "name"), "Apple SPI Keyboard\n")

	kbds, err := discoverKeyboards(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Keyboard{
		{ID: 1, Name: "Apple SPI Keyboard", LEDDir: filepath.Join(dir, "input1::capslock")},
		{ID: 2, Name: "input3::capslock", LEDDir: filepath.Join(dir, "input3::capslock")},
	}
	if len(kbds) != len(want) {
		t.Fatalf("got %d keyboards %v, want %d", len(kbds), kbds, len(want))
	}
	for i := range want {
		if kbds[i] != want[i] {
			t.Errorf("keyboard %d = %+v, want %+v", i, kbds[i], want[i])
		}
	}
}

func TestDiscoverKeyboardsEmpty(t *testing.T) {
	kbds, err := discoverKeyboards(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(kbds) != 0 {
		t.Errorf("got %v, want none", kbds)
	}
}

func TestOpenLED(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "brightness"), "1\n")
	writeFile(t, filepath.Join(dir, "max_brightness"), "2\n")

	led, original, err := openLED(dir)
	if err != nil {
		t.Fatal(err)
	}
	if original != "1" {
		t.Errorf("original = %q, want %q", original, "1")
	}
	if led.onVal != "2" {
		t.Errorf("onVal = %q, want %q (from max_brightness)", led.onVal, "2")
	}

	read := func() string {
		b, err := os.ReadFile(filepath.Join(dir, "brightness"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if err := led.Set(true); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "2" {
		t.Errorf("after Set(true) brightness = %q, want %q", got, "2")
	}
	if err := led.Set(false); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "0" {
		t.Errorf("after Set(false) brightness = %q, want %q", got, "0")
	}
	if err := led.write(original); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "1" {
		t.Errorf("after restore brightness = %q, want original %q", got, "1")
	}
}

func TestOpenLEDMaxBrightnessFallback(t *testing.T) {
	for name, content := range map[string]string{"missing": "", "zero": "0\n", "garbage": "wat\n"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "brightness"), "0\n")
			if content != "" {
				writeFile(t, filepath.Join(dir, "max_brightness"), content)
			}
			led, _, err := openLED(dir)
			if err != nil {
				t.Fatal(err)
			}
			if led.onVal != "1" {
				t.Errorf("onVal = %q, want fallback %q", led.onVal, "1")
			}
		})
	}
}

func TestOpenLEDPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, file modes are not enforced")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "brightness"), "0\n")
	if err := os.Chmod(filepath.Join(dir, "brightness"), 0o444); err != nil {
		t.Fatal(err)
	}
	_, _, err := openLED(dir)
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("err = %v, want fs.ErrPermission (from the startup probe write)", err)
	}
}
