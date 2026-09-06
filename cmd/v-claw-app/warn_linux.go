package main

import (
	"context"
	"log"
	"os/exec"
	"time"
)

// WarnSounds is the closed set of choices. A name from the state file is untrusted
// input and must never reach a filesystem path unchecked.
//
// These are the freedesktop sound theme names, not the macOS ones the state file
// defaults to — LidWarnSound is shared by both platforms and a fresh "Funk" default
// simply falls through to defaultWarnSound here, the same as any other unrecognised
// name would.
var WarnSounds = []string{
	"dialog-warning", "alarm-clock-elapsed", "bell", "complete", "message",
}

const defaultWarnSound = "dialog-warning"

const soundTheme = "/usr/share/sounds/freedesktop/stereo/"

func soundPath(name string) string {
	for _, s := range WarnSounds {
		if s == name {
			return soundTheme + s + ".oga"
		}
	}
	return soundTheme + defaultWarnSound + ".oga"
}

// warnLidClosed plays an audible warning that the machine is still running.
//
// Two short tones rather than one. A single system sound is indistinguishable from any
// other notification, and this one has to be recognised through a closed lid, often
// already inside a bag.
//
// paplay rather than the UI helper: the helper starts lazily and may not be running,
// and this is the one moment where a missing warning matters most.
//
// Preview uses this same function, so what you hear when testing is exactly what plays
// when it matters.
func warnLidClosed(sound string) {
	path := soundPath(sound)

	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := exec.CommandContext(ctx, "paplay", path).Run()
		cancel()
		if err != nil {
			log.Printf("could not play the lid warning: %v", err)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}
