package cmd

import (
	"testing"

	"github.com/toyz/gw/internal/workspace"
)

// TestResolveModule covers the step/dir module lookup: exact path beats an
// ambiguous basename, a unique basename resolves, and ambiguous/missing error.
func TestResolveModule(t *testing.T) {
	mods := []workspace.Module{
		{Path: "example.com/api", Dir: "/w/api"},
		{Path: "example.com/web", Dir: "/w/web"},
		{Path: "example.com/svc/api", Dir: "/w/svc/api"}, // basename "api" collides
	}

	if m, err := resolveModule(mods, "example.com/api"); err != nil || m.Dir != "/w/api" {
		t.Fatalf("exact path: got %+v, err %v", m, err)
	}
	if m, err := resolveModule(mods, "web"); err != nil || m.Path != "example.com/web" {
		t.Fatalf("unique basename: got %+v, err %v", m, err)
	}
	if _, err := resolveModule(mods, "api"); err == nil {
		t.Error("basename 'api' is ambiguous; want error")
	}
	if _, err := resolveModule(mods, "nope"); err == nil {
		t.Error("missing module; want error")
	}
}

// TestParseStep pins the unified-step classifier: a whitespace-free
// module:known-verb is a module op; everything else is a shell command.
func TestParseStep(t *testing.T) {
	cases := []struct {
		step      string
		isModule  bool
		mod, verb string
	}{
		{"api:build", true, "api", "build"},
		{"example.com/api:test", true, "example.com/api", "test"},
		{"sqlc generate", false, "", ""},  // whitespace → shell
		{"npm run dev", false, "", ""},    // whitespace → shell
		{"go build ./...", false, "", ""}, // whitespace → shell
		{"api:deploy", false, "", ""},     // unknown verb → shell
		{"echo", false, "", ""},           // no colon → shell
	}
	for _, c := range cases {
		mod, verb, isMod := parseStep(c.step)
		if isMod != c.isModule || mod != c.mod || verb != c.verb {
			t.Errorf("parseStep(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.step, mod, verb, isMod, c.mod, c.verb, c.isModule)
		}
	}
}
