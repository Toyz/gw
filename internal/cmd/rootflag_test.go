package cmd

import (
	"strings"
	"testing"
)

// -C/--root must be recognized before a "--" separator, but anything after "--"
// is a literal argument (for the command being run or an extension), not gw's
// root flag. Otherwise hooks resolve against the wrong directory and silently
// don't fire (e.g. `gw run -- echo --root /tmp`).
func TestScanRootFlagStopsAtDoubleDash(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-C", "/ws", "run"}, "/ws"},
		{[]string{"--root=/ws", "run"}, "/ws"},
		{[]string{"run", "--", "echo", "--root", "/tmp"}, ""},
		{[]string{"run", "--", "--root=/tmp"}, ""},
		{[]string{"run"}, ""},
	}
	for _, tc := range cases {
		if got := scanRootFlag(tc.args); got != tc.want {
			t.Errorf("scanRootFlag(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestStripRootFlagStopsAtDoubleDash(t *testing.T) {
	// -C before -- is stripped; everything from -- on is forwarded verbatim.
	got := stripRootFlag([]string{"-C", "/ws", "sub", "--", "--root", "/tmp"})
	want := "sub -- --root /tmp"
	if strings.Join(got, " ") != want {
		t.Fatalf("stripRootFlag = %q, want %q", strings.Join(got, " "), want)
	}
}
