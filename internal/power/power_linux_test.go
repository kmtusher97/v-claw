package power

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSupply(t *testing.T, dir, name, typ, online string) {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "type"), []byte(typ+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if online != "" {
		if err := os.WriteFile(filepath.Join(d, "online"), []byte(online+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOnAC(t *testing.T) {
	cases := []struct {
		name  string
		setup func(dir string)
		want  bool
	}{
		{
			name:  "no power_supply class at all",
			setup: func(dir string) {},
			want:  true,
		},
		{
			name: "mains online",
			setup: func(dir string) {
				writeSupply(t, dir, "ACAD", "Mains", "1")
				writeSupply(t, dir, "BAT1", "Battery", "")
			},
			want: true,
		},
		{
			name: "mains offline, on battery",
			setup: func(dir string) {
				writeSupply(t, dir, "ACAD", "Mains", "0")
				writeSupply(t, dir, "BAT1", "Battery", "")
			},
			want: false,
		},
		{
			name: "battery only, no mains node reported",
			setup: func(dir string) {
				writeSupply(t, dir, "BAT1", "Battery", "")
			},
			want: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			c.setup(dir)
			got, err := onAC(dir)
			if err != nil {
				t.Fatalf("onAC: %v", err)
			}
			if got != c.want {
				t.Errorf("onAC(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestInhibitWhat(t *testing.T) {
	cases := []struct {
		opts Options
		want string
	}{
		{Options{}, "idle"},
		{Options{BlockLidSleep: true}, "idle:handle-lid-switch"},
		{Options{KeepDisplayOn: true}, "idle"},
		{Options{BlockLidSleep: true, KeepDisplayOn: true}, "idle:handle-lid-switch"},
	}
	for _, c := range cases {
		if got := inhibitWhat(c.opts); got != c.want {
			t.Errorf("inhibitWhat(%+v) = %q, want %q", c.opts, got, c.want)
		}
	}
}
