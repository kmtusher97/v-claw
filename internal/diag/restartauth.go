package diag

// RestartAuth describes whether restarting this machine actually demands a credential.
//
// This matters more than it looks. The virtual lock is cleared by a restart, which is
// its recovery path for a forgotten password. That is only safe because the restart
// itself lands on a login prompt: the lock's real floor is the account password,
// enforced by the OS rather than by v-claw.
//
// Turn on automatic login and that floor disappears. Restarting then walks straight to
// the desktop, and the virtual lock becomes decoration. Nothing warns the user, so
// v-claw has to.
type RestartAuth struct {
	Required   bool
	FileVault  bool
	AutoLogin  string // the user configured for automatic login, empty when off
	GuestLogin bool
	Warning    string
}
