package power

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

// Linux needs no privilege at all. systemd-logind's Inhibit call blocks idle-triggered
// sleep and the lid-switch action for as long as the returned file descriptor stays
// open, and the kernel closes that fd the moment this process dies — so unlike macOS
// there is no watchdog to write and no daemon to install. Screen blanking is a desktop
// session concern rather than logind's, so it goes through the freedesktop ScreenSaver
// inhibitor on the session bus instead. Both calls were verified live against a real
// logind and a real GNOME session before being wired in here; see docs/spec/09-linux.md.

const (
	login1Dest = "org.freedesktop.login1"
	login1Path = "/org/freedesktop/login1"

	screenSaverDest = "org.freedesktop.ScreenSaver"
	screenSaverPath = "/org/freedesktop/ScreenSaver"

	inhibitWho = "v-claw"
	inhibitWhy = "keeping the machine awake"
)

// sysPowerSupplyDir and lidStateGlob are variables so tests can point them at a fixture
// directory instead of the real machine.
var (
	sysPowerSupplyDir = "/sys/class/power_supply"
	lidStateGlob      = "/proc/acpi/button/lid/*/state"
)

type linuxController struct {
	system    *dbus.Conn
	systemErr error

	// session is opened lazily on first use: a headless or root context may have no
	// session bus at all, and that must not break AC power or lid-close blocking, which
	// do not need it.
	session    *dbus.Conn
	sessionErr error

	// idleLid holds the logind inhibitor. Its "what" is remembered because an Inhibit
	// lock cannot be edited in place — changing what it covers means dropping this one
	// and taking a new one.
	idleLid     *os.File
	idleLidWhat string

	ssCookie uint32
	ssHeld   bool
}

func newController() Controller {
	c := &linuxController{}
	c.system, c.systemErr = dbus.ConnectSystemBus()
	return c
}

func (c *linuxController) OnAC() (bool, error) {
	return onAC(sysPowerSupplyDir)
}

// onAC reads /sys/class/power_supply directly rather than going through upower or
// D-Bus, because it needs no running service and cannot be wrong about what the kernel
// itself already knows.
func onAC(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// No power_supply class at all: a desktop or a container, neither of which
		// will ever have the adapter pulled. Treat that as always plugged in rather
		// than refusing to hold the machine awake.
		return true, nil
	}

	sawMains := false
	for _, e := range entries {
		typ, err := os.ReadFile(filepath.Join(dir, e.Name(), "type"))
		if err != nil || strings.TrimSpace(string(typ)) != "Mains" {
			continue
		}
		sawMains = true
		online, err := os.ReadFile(filepath.Join(dir, e.Name(), "online"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(online)) == "1" {
			return true, nil
		}
	}
	// A machine with only a Mains node, present but reporting anything besides "1", is
	// genuinely on battery. A machine with no Mains node at all is treated as always on,
	// the same as having no power_supply class.
	return !sawMains, nil
}

// LidClosed reports whether the lid is shut. known is false on hardware with no lid, so
// a desktop is never treated as permanently open — matching the darwin behaviour.
//
// The ACPI lid button device only exists when there is a lid, which is what known is
// read from. The state itself comes from logind rather than that file, because logind
// is the same source the Inhibit call above is already trusted to agree with.
func (c *linuxController) LidClosed() (closed, known bool) {
	matches, _ := filepath.Glob(lidStateGlob)
	if len(matches) == 0 || c.system == nil {
		return false, false
	}

	v, err := c.system.Object(login1Dest, login1Path).
		GetProperty("org.freedesktop.login1.Manager.LidClosed")
	if err != nil {
		return false, false
	}
	b, ok := v.Value().(bool)
	if !ok {
		return false, false
	}
	return b, true
}

func (c *linuxController) IdleSeconds() (float64, error) {
	// No virtual lock ships on Linux yet, so nothing needs this. X11 has
	// XScreenSaverQueryInfo and Wayland has the idle-notify protocol, but picking
	// between them is real work with no caller to justify it today. Revisit when the
	// virtual lock's idle trigger is built for this platform.
	return 0, ErrUnsupported
}

func (c *linuxController) Holding() bool { return c.idleLid != nil || c.ssHeld }

func (c *linuxController) Hold(o Options) error {
	var errs []error

	want := inhibitWhat(o)
	if c.idleLid == nil || c.idleLidWhat != want {
		if c.idleLid != nil {
			c.idleLid.Close()
			c.idleLid, c.idleLidWhat = nil, ""
		}
		f, err := c.inhibitIdleLid(want)
		if err != nil {
			errs = append(errs, err)
		} else {
			c.idleLid, c.idleLidWhat = f, want
		}
	}

	switch {
	case o.KeepDisplayOn && !c.ssHeld:
		cookie, err := c.inhibitScreenSaver()
		if err != nil {
			errs = append(errs, err)
		} else {
			c.ssCookie, c.ssHeld = cookie, true
		}
	case !o.KeepDisplayOn && c.ssHeld:
		c.unInhibitScreenSaver()
	}

	return errors.Join(errs...)
}

func (c *linuxController) Release() error {
	var errs []error
	if c.idleLid != nil {
		if err := c.idleLid.Close(); err != nil {
			errs = append(errs, err)
		}
		c.idleLid, c.idleLidWhat = nil, ""
	}
	if c.ssHeld {
		c.unInhibitScreenSaver()
	}
	return errors.Join(errs...)
}

// inhibitWhat maps the platform-neutral Options onto logind's "what" vocabulary. "idle"
// is always requested: it blocks the idle-triggered sleep that is this feature's whole
// point, without touching an explicit "systemctl suspend" the user asks for themselves.
func inhibitWhat(o Options) string {
	what := []string{"idle"}
	if o.BlockLidSleep {
		what = append(what, "handle-lid-switch")
	}
	return strings.Join(what, ":")
}

func (c *linuxController) inhibitIdleLid(what string) (*os.File, error) {
	if c.system == nil {
		return nil, fmt.Errorf("power: system bus unavailable: %w", c.systemErr)
	}
	var fd dbus.UnixFD
	call := c.system.Object(login1Dest, login1Path).Call(
		"org.freedesktop.login1.Manager.Inhibit", 0, what, inhibitWho, inhibitWhy, "block")
	if call.Err != nil {
		return nil, fmt.Errorf("power: logind Inhibit(%s): %w", what, call.Err)
	}
	if err := call.Store(&fd); err != nil {
		return nil, fmt.Errorf("power: logind Inhibit(%s): %w", what, err)
	}
	return os.NewFile(uintptr(fd), "v-claw-inhibit"), nil
}

func (c *linuxController) inhibitScreenSaver() (uint32, error) {
	if err := c.ensureSession(); err != nil {
		return 0, err
	}
	var cookie uint32
	call := c.session.Object(screenSaverDest, screenSaverPath).
		Call("org.freedesktop.ScreenSaver.Inhibit", 0, inhibitWho, inhibitWhy)
	if call.Err != nil {
		return 0, fmt.Errorf("power: ScreenSaver.Inhibit: %w", call.Err)
	}
	if err := call.Store(&cookie); err != nil {
		return 0, fmt.Errorf("power: ScreenSaver.Inhibit: %w", err)
	}
	return cookie, nil
}

func (c *linuxController) unInhibitScreenSaver() {
	if c.session != nil {
		c.session.Object(screenSaverDest, screenSaverPath).
			Call("org.freedesktop.ScreenSaver.UnInhibit", 0, c.ssCookie)
	}
	c.ssCookie, c.ssHeld = 0, false
}

func (c *linuxController) ensureSession() error {
	if c.session != nil {
		return nil
	}
	if c.sessionErr != nil {
		return c.sessionErr
	}
	c.session, c.sessionErr = dbus.ConnectSessionBus()
	if c.sessionErr != nil {
		c.sessionErr = fmt.Errorf("power: session bus unavailable: %w", c.sessionErr)
		return c.sessionErr
	}
	return nil
}

func (c *linuxController) Capabilities() Caps {
	if c.system == nil {
		return Caps{ExplainUnavailable: fmt.Sprintf(
			"cannot reach the system D-Bus (%v); lid-close blocking needs systemd-logind", c.systemErr)}
	}
	// A live property read is the same call LidClosed already relies on, so this is
	// also proof that Inhibit itself will work, not just that a socket connected.
	if _, err := c.system.Object(login1Dest, login1Path).
		GetProperty("org.freedesktop.login1.Manager.LidClosed"); err != nil {
		return Caps{ExplainUnavailable: fmt.Sprintf("systemd-logind is not answering: %v", err)}
	}
	return Caps{LidBlockAvailable: true}
}
