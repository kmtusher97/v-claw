//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/kamrul1157024/v-claw/internal/paths"
	"github.com/kamrul1157024/v-claw/internal/state"
)

// lockReset forgets the virtual lock password. It is the escape hatch for a forgotten
// one, and it gives nothing away: the virtual lock is a window, not a security
// boundary, so quitting v-claw already removes it.
func lockReset() error {
	out, err := exec.Command("/usr/bin/security", "delete-generic-password",
		"-s", "com.vclaw.virtual-lock").CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "could not be found") {
			fmt.Println("no virtual lock password was set")
			return nil
		}
		return fmt.Errorf("could not clear the password: %s", out)
	}

	// Leaving the policy pointing at a password that no longer exists would make the
	// lock fail open silently. Say so, and move it back to the honest setting.
	s, lerr := state.Load(paths.StateFile())
	if lerr == nil && s.Lock.Policy == state.PolicyPassword {
		s.Lock.Policy = state.PolicyNone
		if err := state.Save(paths.StateFile(), s); err != nil {
			return err
		}
		fmt.Println("password cleared; the virtual lock is back to \"any key unlocks\"")
		return nil
	}
	fmt.Println("password cleared")
	return nil
}
