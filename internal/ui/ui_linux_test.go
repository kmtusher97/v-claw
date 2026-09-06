package ui

import (
	"reflect"
	"testing"
)

func TestFormEvents(t *testing.T) {
	base := State{Mode: "auto", BlockLidSleep: true, KeepDisplayOn: true}

	cases := []struct {
		name string
		out  string
		want []Event
	}{
		{
			name: "nothing changed, timer left blank",
			out:  "auto|on|on|\n",
			want: nil,
		},
		{
			name: "mode changed",
			out:  "always|on|on|",
			want: []Event{{Ev: "setMode", Mode: "always"}},
		},
		{
			name: "both flags flipped off",
			out:  "auto|off|off|",
			want: []Event{
				{Ev: "setFlag", Flag: "block_lid_sleep", Value: false},
				{Ev: "setFlag", Flag: "keep_display_on", Value: false},
			},
		},
		{
			name: "timer set",
			out:  "auto|on|on|15",
			want: []Event{{Ev: "setTimer", Seconds: 900}},
		},
		{
			name: "timer explicitly cleared with zero",
			out:  "auto|on|on|0",
			want: []Event{{Ev: "setTimer", Seconds: 0}},
		},
		{
			name: "garbage timer text is ignored rather than crashing",
			out:  "auto|on|on|not a number",
			want: nil,
		},
		{
			name: "malformed zenity output produces no events",
			out:  "auto|on|on",
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := formEvents(base, c.out)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("formEvents(%q) = %+v, want %+v", c.out, got, c.want)
			}
		})
	}
}

func TestFirstThenRest(t *testing.T) {
	got := firstThenRest("always", "off", "auto", "always")
	want := []string{"always", "off", "auto"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("firstThenRest = %v, want %v", got, want)
	}
}

func TestOnOff(t *testing.T) {
	if got := onOff(true); !reflect.DeepEqual(got, []string{"on", "off"}) {
		t.Errorf("onOff(true) = %v", got)
	}
	if got := onOff(false); !reflect.DeepEqual(got, []string{"off", "on"}) {
		t.Errorf("onOff(false) = %v", got)
	}
}
