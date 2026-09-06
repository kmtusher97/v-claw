//go:build !darwin

package main

// reopen exists so main's select loop compiles on every platform. It never fires here:
// the "reopen" Apple event has no equivalent outside macOS, and a tray icon click does
// not need one — clicking it opens the tray menu, not the app.
var reopen = make(chan struct{})

func installReopenHandler() {}
