package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasIssue(issues []Issue, sev Severity, substr string) bool {
	for _, i := range issues {
		if i.Severity == sev && strings.Contains(i.Msg, substr) {
			return true
		}
	}
	return false
}

func TestDiagnose(t *testing.T) {
	root := t.TempDir()
	writeMod(t, root, "svc-a", "module example.com/svc-a\n\ngo 1.25.0\n\nrequire github.com/pkg/errors v0.9.1\n")
	writeMod(t, root, "svc-b", "module example.com/svc-b\n\ngo 1.24.0\n\nrequire github.com/pkg/errors v0.8.0\nreplace github.com/x/y => ../y\n")

	// go.work missing entirely.
	mods, _ := Discover(root, Config{})
	if is := Diagnose(root, Config{}, mods); !hasIssue(is, SevError, "no go.work") {
		t.Fatalf("expected missing go.work error, got %+v", is)
	}

	// go.work listing a stale/orphaned use dir + missing svc-b.
	os.WriteFile(filepath.Join(root, "go.work"),
		[]byte("go 1.25.0\n\nuse (\n\t./svc-a\n\t./ghost\n)\n"), 0o644)
	is := Diagnose(root, Config{}, mods)
	if !hasIssue(is, SevError, "./ghost") {
		t.Fatalf("expected orphaned use error, got %+v", is)
	}
	if !hasIssue(is, SevWarn, "svc-b") {
		t.Fatalf("expected missing-module warning, got %+v", is)
	}
	if !hasIssue(is, SevWarn, "github.com/pkg/errors") {
		t.Fatalf("expected version mismatch warning, got %+v", is)
	}
	if !hasIssue(is, SevWarn, "go directive") {
		t.Fatalf("expected go directive warning, got %+v", is)
	}
	if !hasIssue(is, SevInfo, "replace directive") {
		t.Fatalf("expected un-hoisted replace info, got %+v", is)
	}
}
