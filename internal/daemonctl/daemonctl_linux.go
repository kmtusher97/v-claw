// Package daemonctl reports whether guaranteed lid blocking is available.
//
// On Linux there is no privileged helper to check: logind's Inhibit needs no privilege
// at all, so tier 0 and tier 1 are the same tier. This file exists only to answer
// "is logind actually there", which is false on a non-systemd distribution.
package daemonctl

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// Status reports whether lid blocking is available, and if not, why. The reason is
// shown to the user, so it must name the cause and the fix.
func Status() (ok bool, reason string) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return false, fmt.Sprintf(
			"cannot reach the system D-Bus (%v); lid-close blocking needs systemd-logind", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1"))
	if _, err := obj.GetProperty("org.freedesktop.login1.Manager.LidClosed"); err != nil {
		return false, fmt.Sprintf("systemd-logind is not answering: %v", err)
	}
	return true, ""
}
