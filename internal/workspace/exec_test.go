package workspace

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestRunAcrossParallelStreamSeparation guards the fix that parallel mode must
// honor the caller's separate stdout/stderr writers (it once folded both into
// stdout, so diagnostics escaped a `2>` redirect only when --parallel was set).
func TestRunAcrossParallelStreamSeparation(t *testing.T) {
	dir := t.TempDir()
	jobs := []Job{{
		Module: Module{Path: "m", Dir: dir},
		Argv:   []string{"sh", "-c", "echo OUT; echo ERR 1>&2"},
	}}
	var out, err bytes.Buffer
	RunAcross(context.Background(), jobs, ExecOpts{Parallel: true, Stdout: &out, Stderr: &err})

	if !strings.Contains(out.String(), "OUT") {
		t.Errorf("stdout missing OUT: %q", out.String())
	}
	if strings.Contains(out.String(), "ERR") {
		t.Errorf("stdout leaked stderr: %q", out.String())
	}
	if !strings.Contains(err.String(), "ERR") {
		t.Errorf("stderr missing ERR: %q", err.String())
	}
	if strings.Contains(err.String(), "OUT") {
		t.Errorf("stderr leaked stdout: %q", err.String())
	}
}
