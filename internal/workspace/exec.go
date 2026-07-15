package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// ExecOpts configures RunAcross.
type ExecOpts struct {
	// Parallel runs modules concurrently (bounded by MaxParallel) instead of serially.
	Parallel bool
	// MaxParallel caps concurrency when Parallel is set (<=0 means GOMAXPROCS).
	MaxParallel int
	// ContinueOnError keeps running remaining modules after one fails (serial only).
	ContinueOnError bool
	// Env holds KEY=VALUE overrides layered on top of the ambient environment for
	// every module command. Nil leaves the child environment inherited as-is.
	Env []string
	// Stdout/Stderr receive command output. Defaults to os.Stdout/os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// ModuleResult is the outcome of running a command in one module.
type ModuleResult struct {
	Module   Module
	ExitCode int
	Err      error
	Duration time.Duration
}

// Failed reports whether the command failed for this module.
func (r ModuleResult) Failed() bool { return r.ExitCode != 0 || r.Err != nil }

var errSkipped = fmt.Errorf("skipped after earlier failure")

// RunAcross runs argv in every module's directory and returns per-module results.
// In serial mode output streams live; in parallel mode each module's output is
// buffered and flushed with a header so lines from different modules don't interleave.
func RunAcross(ctx context.Context, mods []Module, argv []string, opts ExecOpts) []ModuleResult {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	results := make([]ModuleResult, len(mods))

	if !opts.Parallel {
		for i, m := range mods {
			fmt.Fprintf(opts.Stdout, "== %s ==\n", m.Path)
			results[i] = runOne(ctx, m, argv, opts.Env, opts.Stdout, opts.Stderr)
			if results[i].Failed() && !opts.ContinueOnError {
				for j := i + 1; j < len(mods); j++ {
					results[j] = ModuleResult{Module: mods[j], ExitCode: -1, Err: errSkipped}
				}
				break
			}
		}
		return results
	}

	limit := opts.MaxParallel
	if limit <= 0 {
		limit = runtime.GOMAXPROCS(0)
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var flushMu sync.Mutex
	for i, m := range mods {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, m Module) {
			defer wg.Done()
			defer func() { <-sem }()
			buf := &syncBuffer{}
			results[i] = runOne(ctx, m, argv, opts.Env, buf, buf)
			flushMu.Lock()
			fmt.Fprintf(opts.Stdout, "== %s ==\n", m.Path)
			_, _ = opts.Stdout.Write(buf.Bytes())
			flushMu.Unlock()
		}(i, m)
	}
	wg.Wait()
	return results
}

func runOne(ctx context.Context, m Module, argv, env []string, stdout, stderr io.Writer) ModuleResult {
	start := time.Now()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = m.Dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	res := ModuleResult{Module: m, Duration: time.Since(start)}
	if err != nil {
		res.Err = err
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
	}
	return res
}

// WorstExit returns a non-zero exit code if any module failed, else 0.
func WorstExit(results []ModuleResult) int {
	worst := 0
	for _, r := range results {
		switch {
		case r.ExitCode > worst:
			worst = r.ExitCode
		case r.ExitCode < 0 && worst == 0:
			worst = 1
		}
	}
	return worst
}

// PrintSummary writes a per-module ok/fail + duration table to w.
func PrintSummary(w io.Writer, results []ModuleResult) {
	fmt.Fprintln(w, "\nSummary:")
	failed := 0
	for _, r := range results {
		status := "ok"
		if r.Failed() {
			status = "FAIL"
			failed++
		}
		fmt.Fprintf(w, "  %-4s  %-50s  %s\n", status, r.Module.Path, r.Duration.Round(time.Millisecond))
	}
	fmt.Fprintf(w, "%d module(s), %d failed\n", len(results), failed)
}

// syncBuffer is an io.Writer safe for concurrent writes (stdout+stderr of one command).
type syncBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.buf))
	copy(out, b.buf)
	return out
}
