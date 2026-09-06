//go:build darwin

package diag

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

func CheckRestartAuth() RestartAuth {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r := RestartAuth{
		AutoLogin:  autoLoginUser(ctx),
		FileVault:  fileVaultOn(ctx),
		GuestLogin: guestEnabled(ctx),
	}
	r.Required = r.AutoLogin == ""

	switch {
	case !r.Required:
		r.Warning = "automatic login is on for \"" + r.AutoLogin +
			"\", so restarting reaches the desktop without a password. " +
			"That makes the virtual lock trivial to bypass."
	case r.GuestLogin:
		r.Warning = "the guest account is enabled, so a restart offers a way in " +
			"without your password. The virtual lock is weaker than it looks."
	}
	return r
}

func autoLoginUser(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read",
		"/Library/Preferences/com.apple.loginwindow", "autoLoginUser").Output()
	if err != nil {
		// The key is absent when automatic login is off, and `defaults` exits non-zero.
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fileVaultOn(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "/usr/bin/fdesetup", "status").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "FileVault is On")
}

func guestEnabled(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read",
		"/Library/Preferences/com.apple.loginwindow", "GuestEnabled").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}
