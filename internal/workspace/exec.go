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
	// Stdout/Stderr receive command output. Defaults to os.Stdout/os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
	// Header renders the heading printed before each module's output. Lets the
	// caller style it; nil falls back to "== <path> ==".
	Header func(module string) string
}

// Job is one module command: the argv to run in the module's directory and the
// KEY=VALUE environment overrides for it. Per-module jobs let callers vary the
// command and environment across modules (e.g. per-module build providers).
type Job struct {
	Module Module
	Argv   []string
	Env    []string // overrides appended to os.Environ(); nil inherits as-is
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

// RunAcross runs each job in its module's directory and returns per-module
// results. In serial mode output streams live; in parallel mode each module's
// output is buffered and flushed with a header so lines from different modules
// don't interleave.
func RunAcross(ctx context.Context, jobs []Job, opts ExecOpts) []ModuleResult {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	header := opts.Header
	if header == nil {
		header = func(m string) string { return "== " + m + " ==" }
	}
	results := make([]ModuleResult, len(jobs))

	if !opts.Parallel {
		for i, j := range jobs {
			fmt.Fprintln(opts.Stdout, header(j.Module.Path))
			results[i] = runOne(ctx, j, opts.Stdout, opts.Stderr)
			if results[i].Failed() && !opts.ContinueOnError {
				for k := i + 1; k < len(jobs); k++ {
					results[k] = ModuleResult{Module: jobs[k].Module, ExitCode: -1, Err: errSkipped}
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
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, j Job) {
			defer wg.Done()
			defer func() { <-sem }()
			// Buffer each stream separately so parallel mode preserves the
			// caller's stdout/stderr split (serial mode already does via runOne);
			// folding both into one writer would send diagnostics to stdout.
			var outBuf, errBuf syncBuffer
			results[i] = runOne(ctx, j, &outBuf, &errBuf)
			flushMu.Lock()
			fmt.Fprintln(opts.Stdout, header(j.Module.Path))
			_, _ = opts.Stdout.Write(outBuf.Bytes())
			_, _ = opts.Stderr.Write(errBuf.Bytes())
			flushMu.Unlock()
		}(i, j)
	}
	wg.Wait()
	return results
}

func runOne(ctx context.Context, j Job, stdout, stderr io.Writer) ModuleResult {
	start := time.Now()
	cmd := exec.CommandContext(ctx, j.Argv[0], j.Argv[1:]...)
	cmd.Dir = j.Module.Dir
	if len(j.Env) > 0 {
		cmd.Env = append(os.Environ(), j.Env...)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	res := ModuleResult{Module: j.Module, Duration: time.Since(start)}
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
