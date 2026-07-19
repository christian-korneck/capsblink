# capsblink - use your keyboard's caps-lock LED for fun and profit

A tiny Linux CLI tool to flash / [blinken](https://en.wikipedia.org/wiki/Blinkenlights) the keyboard's caps lock LED. Intended for signalling events like "email received", "monitoring alert" or "Claude is waiting for input".

Needs to run as root (or with sudo) - or alternatively requires a one-time setup of a udev rule, see infos below.

Mostly vibe-coded with Claude Code.

## Usage

```
capsblink --help
Usage: capsblink [flags]

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
```

Examples:

```bash
# blink the capslock LED on the default keyboard continously (press CTRL+C to stop)
sudo capsblink

# you might want a specific keyboard. This lists the available keyboards
$ capsblink --list
1       Apple SPI Keyboard
2       Dell KB216 Wired Keyboard

# and then we we can blink on a specific keyboard (here the Dell one):
sudo capsblink --device 2

# there are several built-in blink patterns:
sudo capsblink # standard pattern: regular continuous blink
sudo capsblink --pattern slow # slow continuous blink
sudo capsblink --p fast # fast continuous flashing
sudo capsblink --p twoshort # two short flashes, long pause, repeat

# we can limit the number of times the pattern gets repeated
# example: blink two times, then exit.
sudo capsblink -n 2 

# and you can also define your own, custom pattern:
# - there are keywords for pausing: short break `(sb)`, medium break `(mb)`, long break `(lb)`
# - and keywords for flashes: short flash `(sf)`, medium flash `(mf)`, long flash `(lf)`
#
# so three short flashes, followed by a long pause (and then repeat) would look like:
sudo capsblink --pattern "(sf)(sf)(sf)(lb)"
```

## Running without root (udev rule)

To be able to run `capsblink` without `sudo`, you can set up a udev rule like:

```bash
sudo tee /etc/udev/rules.d/99-capslock-led.rules <<'EOF'
ACTION=="add", SUBSYSTEM=="leds", KERNEL=="input*::capslock", RUN+="/bin/chmod 0666 /sys/class/leds/%k/brightness"
EOF

sudo udevadm control --reload
sudo udevadm trigger --action=add -s leds
```

## Usage Example: Blink the LED when Claude Code is waiting for input

with the udev rule in place (see above), add these hooks to `~/.claude/settings.json`:
(modify the parameters to your liking)

```
  "hooks": {
  "Stop": [{"hooks":[{"type":"command","command":"capsblink -n 2 -p twoshort >/dev/null 2>&1 &"}]}],
  "Notification": [{"hooks":[{"type":"command","command":"capsblink -n 2 -p twoshort >/dev/null 2>&1 &"}]}]
  }
```
