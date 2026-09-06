// Package ui drives whatever v-claw uses to draw windows beyond the tray menu.
//
// On macOS that is a native AppKit helper process, driven over a stdin/stdout JSON
// protocol because Go has no good AppKit binding. On Linux there is no equivalent
// window yet, so a much smaller implementation drives simple dialogs directly.
// State and Event are shared by both, because the caller — cmd/v-claw-app — must not
// need to know which platform it is running on.
package ui

// State is what a settings window needs to draw itself. On the darwin helper, the field
// names are also the wire format.
type State struct {
	Mode           string `json:"mode"`
	BlockLidSleep  bool   `json:"blockLidSleep"`
	KeepDisplayOn  bool   `json:"keepDisplayOn"`
	WarnOnLidClose bool   `json:"warnOnLidClose"`
	LidWarnSound   string `json:"lidWarnSound"`
	LidWarnEvery   int    `json:"lidWarnEvery"`
	ShowInDock     bool   `json:"showInDock"`

	// OnBatteryAwake means v-claw is holding the machine awake with no adapter
	// attached, so the battery is draining towards flat with nothing to stop it.
	OnBatteryAwake   bool   `json:"onBatteryAwake"`
	ExpiresInSeconds *int   `json:"expiresInSeconds"`
	OnAC             bool   `json:"onAC"`
	Holding          bool   `json:"holding"`
	Tier             string `json:"tier"`
	StatusLine       string `json:"statusLine"`
	LidHint          string `json:"lidHint"`
	LockEnabled      bool   `json:"lockEnabled"`
	LockPolicy       string `json:"lockPolicy"`
	LockIdleMinutes  int    `json:"lockIdleMinutes"`
	HotkeyEnabled    bool   `json:"hotkeyEnabled"`

	// RestartAuthWarning is non-empty when a restart would reach the desktop without
	// a password. The virtual lock leans on restart being authenticated, so when that
	// stops being true the user has to be told.
	RestartAuthWarning string `json:"restartAuthWarning"`
}

// Event is what the user did. Pointer fields distinguish "not sent" from "sent false",
// because the lock controls report only the field that changed.
type Event struct {
	Ev          string `json:"ev"`
	Mode        string `json:"mode"`
	Flag        string `json:"flag"`
	Value       bool   `json:"value"`
	Seconds     int    `json:"seconds"`
	Policy      string `json:"policy"`
	Enabled     *bool  `json:"enabled"`
	IdleMinutes *int   `json:"idleMinutes"`
	Message     string `json:"message"`
	Sound       string `json:"sound"`

	// Reported once when a password lock engages. Diagnostic only.
	KeyWindow    bool `json:"keyWindow"`
	CanBecomeKey bool `json:"canBecomeKey"`
}
