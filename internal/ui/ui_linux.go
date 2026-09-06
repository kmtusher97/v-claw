package ui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// This is deliberately the "basic settings window" rather than a port of the darwin
// helper: there is no single native toolkit every Linux desktop shares the way every
// Mac shares AppKit, and the tray menu already covers every setting on its own. zenity
// ships on GNOME by default and is packaged everywhere else, and its --forms dialog is
// enough to edit the handful of fields that matter without taking on a GTK build
// dependency for a v1. The virtual lock, the permissions window and the Dock icon have
// no Linux equivalent yet and are left out rather than faked.
//
// A zenity dialog cannot be pushed to once it is open, unlike the darwin helper. Show
// opens a fresh dialog seeded with the state at that moment; Push is a no-op.

type UI struct {
	events chan Event

	mu  sync.Mutex
	cmd *exec.Cmd
}

func New() *UI { return &UI{events: make(chan Event, 16)} }

// Available reports whether zenity is on PATH, so the tray menu can grey out the
// settings item rather than fail silently when it is clicked.
func Available() bool { _, err := exec.LookPath("zenity"); return err == nil }

func (u *UI) Events() <-chan Event { return u.events }

func (u *UI) Show(s State) error {
	if !Available() {
		return fmt.Errorf("ui: zenity is not installed (try: sudo apt install zenity)")
	}
	go u.runForm(s)
	return nil
}

// Push cannot be honoured: a zenity dialog cannot be updated once it is showing. The
// window reflects state as of the moment it was opened, which is the accepted trade-off
// of the basic settings window.
func (u *UI) Push(State) error { return nil }

func (u *UI) Hide() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.cmd != nil && u.cmd.Process != nil {
		_ = u.cmd.Process.Kill()
	}
	return nil
}

// Dock has no meaning outside macOS.
func (u *UI) Dock(bool) error { return nil }

// Permissions has no meaning outside macOS: Linux has no TCC-style prompt for v-claw to
// walk someone through.
func (u *UI) Permissions(State) error { return nil }

func (u *UI) Unlock() error { return nil }

func (u *UI) Diagnostics(text string) error {
	if !Available() {
		return fmt.Errorf("ui: zenity is not installed (try: sudo apt install zenity)")
	}
	cmd := exec.Command("zenity", "--text-info",
		"--title=v-claw diagnostics", "--width=700", "--height=500")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		fmt.Fprint(stdin, text)
		stdin.Close()
		_ = cmd.Wait()
	}()
	return nil
}

// Lock is not implemented on Linux yet: there is no single shielding mechanism that
// works the same way across X11, Wayland, GNOME and KDE, and a half-built lock is worse
// than none. Returning an error here is what keeps the caller from ever marking the
// lock engaged, so nothing claims a guarantee that is not in place.
func (u *UI) Lock(string, string) error {
	return fmt.Errorf("the virtual lock is not implemented on Linux yet")
}

func (u *UI) Notify(title, body string) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("ui: notify-send is not installed")
	}
	return exec.Command("notify-send", title, body).Start()
}

func (u *UI) Close() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.cmd != nil && u.cmd.Process != nil {
		_ = u.cmd.Process.Kill()
	}
}

// runForm shows the settings dialog and turns whatever changed into the same Event
// vocabulary the tray menu already produces, so cmd/v-claw-app needs no Linux-specific
// handling on the receiving end.
func (u *UI) runForm(s State) {
	args := []string{
		"--forms",
		"--title=v-claw settings",
		"--text=" + s.StatusLine,
		"--add-combo=Mode", "--combo-values=" + strings.Join(firstThenRest(s.Mode, "off", "auto", "always"), "|"),
		"--add-combo=Block lid sleep", "--combo-values=" + strings.Join(onOff(s.BlockLidSleep), "|"),
		"--add-combo=Keep display on", "--combo-values=" + strings.Join(onOff(s.KeepDisplayOn), "|"),
		"--add-entry=Awake for minutes (blank = leave unchanged, 0 = no timer)",
	}

	cmd := exec.Command("zenity", args...)
	u.mu.Lock()
	u.cmd = cmd
	u.mu.Unlock()

	out, err := cmd.Output()
	u.mu.Lock()
	u.cmd = nil
	u.mu.Unlock()

	if err != nil {
		// Cancelled, closed, or killed by Hide/Close. All the same to the caller.
		u.emit(Event{Ev: "windowClosed"})
		return
	}

	for _, ev := range formEvents(s, string(out)) {
		u.emit(ev)
	}
	u.emit(Event{Ev: "windowClosed"})
}

// formEvents is the pure half of runForm: given the state the form was opened with and
// zenity's raw stdout, it decides what changed. Kept separate from the exec.Command
// call so the decision can be tested without a display to click through.
func formEvents(s State, rawOutput string) []Event {
	fields := strings.Split(strings.TrimRight(rawOutput, "\n"), "|")
	if len(fields) != 4 {
		return nil
	}
	mode, blockLid, keepDisplay, timer := fields[0], fields[1], fields[2], fields[3]

	var evs []Event
	if mode != s.Mode {
		evs = append(evs, Event{Ev: "setMode", Mode: mode})
	}
	if want := blockLid == "on"; want != s.BlockLidSleep {
		evs = append(evs, Event{Ev: "setFlag", Flag: "block_lid_sleep", Value: want})
	}
	if want := keepDisplay == "on"; want != s.KeepDisplayOn {
		evs = append(evs, Event{Ev: "setFlag", Flag: "keep_display_on", Value: want})
	}
	if timer = strings.TrimSpace(timer); timer != "" {
		if minutes, err := strconv.Atoi(timer); err == nil && minutes >= 0 {
			evs = append(evs, Event{Ev: "setTimer", Seconds: minutes * 60})
		}
	}
	return evs
}

func (u *UI) emit(ev Event) {
	select {
	case u.events <- ev:
	default:
	}
}

// firstThenRest puts the current value first, which is what zenity pre-selects — it has
// no flag to choose a default for a --forms combo.
func firstThenRest(cur string, all ...string) []string {
	out := []string{cur}
	for _, v := range all {
		if v != cur {
			out = append(out, v)
		}
	}
	return out
}

func onOff(cur bool) []string {
	if cur {
		return []string{"on", "off"}
	}
	return []string{"off", "on"}
}
