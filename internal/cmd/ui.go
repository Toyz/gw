package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// printer is gw's single output path. Commands build one with newPrinter(cmd)
// and use its vocabulary (ok/warn/fail/info/step/hint and finding/result) so
// every command speaks the same visual language. It respects a command's
// configured writers and auto-disables color on non-terminals.
type printer struct {
	out io.Writer
	err io.Writer
	s   styler // stdout styler
	es  styler // stderr styler
}

// newPrinter builds a printer bound to a command's stdout/stderr.
func newPrinter(cmd *cobra.Command) *printer {
	out, errw := cmd.OutOrStdout(), cmd.ErrOrStderr()
	return &printer{out: out, err: errw, s: newStyler(out), es: newStyler(errw)}
}

// ── raw output ──

// printf/println write unstyled output to stdout — for data (module paths,
// tables) that must stay machine-readable.
func (p *printer) printf(format string, a ...any) { fmt.Fprintf(p.out, format, a...) }
func (p *printer) println(a ...any)               { fmt.Fprintln(p.out, a...) }

// json writes v to stdout as indented JSON — the shared path for --json output.
func (p *printer) json(v any) error {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// warnf writes a non-fatal warning to stderr (a side-channel note, e.g. a hook
// error that shouldn't abort the command).
func (p *printer) warnf(format string, a ...any) {
	fmt.Fprintf(p.err, "%s %s\n", p.es.yellow("⚠"), fmt.Sprintf(format, a...))
}

// Out / Err expose the raw writers for helpers that take an io.Writer
// (e.g. ext.RunHook, or the streamed output of RunAcross).
func (p *printer) Out() io.Writer { return p.out }
func (p *printer) Err() io.Writer { return p.err }

// ── status vocabulary (stdout) ──

func (p *printer) ok(format string, a ...any) { p.line(p.s.green("✓"), format, a...) }
func (p *printer) warn(format string, a ...any) {
	p.line(p.s.yellow("⚠"), format, a...)
}
func (p *printer) fail(format string, a ...any) { p.line(p.s.red("✗"), format, a...) }
func (p *printer) info(format string, a ...any) { p.line(p.s.dim("·"), format, a...) }
func (p *printer) step(format string, a ...any) { p.line(p.s.cyan("→"), format, a...) }

func (p *printer) line(glyph, format string, a ...any) {
	fmt.Fprintf(p.out, "%s %s\n", glyph, fmt.Sprintf(format, a...))
}

// hint prints a dim, indented "help: <text>" line under a finding or message.
func (p *printer) hint(format string, a ...any) {
	fmt.Fprintf(p.out, "  %s %s\n",
		p.s.dim("help:"), p.s.dim(fmt.Sprintf(format, a...)))
}

// ── leveled findings (doctor / lint / verify) ──

type level int

const (
	lvlInfo level = iota
	lvlWarn
	lvlError
)

// finding is one leveled report line with an optional actionable hint.
type finding struct {
	level level
	msg   string
	hint  string
}

// printFinding renders one finding with its level glyph and (optional) hint.
func (p *printer) printFinding(f finding) {
	switch f.level {
	case lvlError:
		p.fail("%s", f.msg)
	case lvlWarn:
		p.warn("%s", f.msg)
	default:
		p.info("%s", f.msg)
	}
	if f.hint != "" {
		p.hint("%s", f.hint)
	}
}

// result closes out a leveled report: it returns the exit error given the
// tallied counts. Clean (all zero) prints "✓ okMsg" and returns nil. Errors —
// or warnings under --strict — return a cmdError carrying the summary (the
// top-level handler renders it). Otherwise it prints a non-fatal summary and
// returns nil.
func (p *printer) result(errs, warns, infos int, strict bool, okMsg string) error {
	if errs+warns+infos == 0 {
		p.ok("%s", okMsg)
		return nil
	}
	summary := severitySummary(errs, warns, infos)
	if errs > 0 {
		return failf("%s", summary)
	}
	if strict && warns > 0 {
		return failf("%s (--strict)", summary)
	}
	if warns > 0 {
		p.warn("%s", summary)
	} else {
		p.info("%s", summary)
	}
	return nil
}

// report prints a slice of findings and their summary in one call — for
// commands with no extra output between the findings and the tally.
func (p *printer) report(findings []finding, strict bool, okMsg string) error {
	var errs, warns, infos int
	for _, f := range findings {
		p.printFinding(f)
		switch f.level {
		case lvlError:
			errs++
		case lvlWarn:
			warns++
		default:
			infos++
		}
	}
	if len(findings) > 0 {
		p.println()
	}
	return p.result(errs, warns, infos, strict, okMsg)
}

// severitySummary renders "2 errors, 1 warning" from the counts, omitting zeros.
func severitySummary(errs, warns, infos int) string {
	var parts []string
	if errs > 0 {
		parts = append(parts, plural(errs, "error"))
	}
	if warns > 0 {
		parts = append(parts, plural(warns, "warning"))
	}
	if infos > 0 {
		parts = append(parts, plural(infos, "note"))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
