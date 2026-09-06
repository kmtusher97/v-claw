// Package diag builds the report shown by `v-claw diagnose`.
package diag

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kamrul1157024/v-claw/internal/daemonctl"
	"github.com/kamrul1157024/v-claw/internal/paths"
	"github.com/kamrul1157024/v-claw/internal/power"
	"github.com/kamrul1157024/v-claw/internal/state"
)

func Report(version string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }

	p("v-claw diagnostics")
	p("  version        %s", version)
	p("  distro         %s", distro())
	p("  kernel         %s", run(ctx, "uname", "-r"))
	p("  session        %s", sessionType())
	p("")

	ok, why := daemonctl.Status()
	p("tier             %s", tierName(ok))
	if !ok {
		p("  reason         %s", why)
	}

	pow := power.New()
	src := "Battery"
	if onAC, err := pow.OnAC(); err != nil {
		src = "unknown: " + err.Error()
	} else if onAC {
		src = "AC power"
	}
	p("power source     %s", src)

	s, serr := state.Load(paths.StateFile())
	switch {
	case serr != nil && os.IsNotExist(serr):
		p("state            not written yet (the app has not run)")
	case serr != nil:
		p("state            UNUSABLE: %v", serr)
	default:
		p("mode             %s", s.Mode)
		p("block lid sleep  %v", s.BlockLidSleep)
		p("keep display on  %v", s.KeepDisplayOn)
		p("virtual lock     not implemented on Linux yet")
		if s.Stale(time.Now()) {
			p("  ! heartbeat is stale; v-claw is not holding anything right now")
		}
	}
	p("")

	if !ok {
		p("warnings")
		p("  ! %s", why)
		return b.String()
	}
	p("warnings")
	p("  none")
	return b.String()
}

func distro() string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if name, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(name, `"`)
		}
	}
	return "unknown"
}

// sessionType names X11 or Wayland, because the answer changes what v-claw can promise:
// a shielding window is only ever possible on X11, and Wayland gives a client no way to
// cover the screen at all.
func sessionType() string {
	if s := os.Getenv("XDG_SESSION_TYPE"); s != "" {
		return s
	}
	return "unknown"
}

func tierName(ok bool) string {
	if ok {
		return "full"
	}
	return "basic"
}

func run(ctx context.Context, bin string, args ...string) string {
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
