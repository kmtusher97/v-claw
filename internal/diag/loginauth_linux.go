package diag

// CheckRestartAuth has nothing to warn about yet: the virtual lock this guards has not
// shipped on Linux, so there is no guarantee here for a bypassed login to weaken.
// Detecting automatic login is also not one question on Linux — GDM, SDDM and LightDM
// each keep it in a different config file — so this is deferred until the lock itself
// is built for this platform, rather than guessed at now.
func CheckRestartAuth() RestartAuth {
	return RestartAuth{Required: true}
}
