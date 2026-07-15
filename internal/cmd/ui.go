package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// printer is gw's single output path: commands build one with newPrinter(cmd)
// and call its methods instead of sprinkling fmt.Fprintf. It respects a
// command's configured writers, so output stays redirectable and testable.
type printer struct {
	out io.Writer
	err io.Writer
}

// newPrinter builds a printer bound to a command's stdout/stderr.
func newPrinter(cmd *cobra.Command) *printer {
	return &printer{out: cmd.OutOrStdout(), err: cmd.ErrOrStderr()}
}

// printf writes formatted output to stdout.
func (p *printer) printf(format string, a ...any) { fmt.Fprintf(p.out, format, a...) }

// println writes a line to stdout.
func (p *printer) println(a ...any) { fmt.Fprintln(p.out, a...) }

// warnf writes a non-fatal "gw: " warning to stderr.
func (p *printer) warnf(format string, a ...any) {
	fmt.Fprintf(p.err, "gw: "+format+"\n", a...)
}

// Out / Err expose the underlying writers for lower-level helpers that take an
// io.Writer (e.g. workspace.PrintSummary, ext.RunHook).
func (p *printer) Out() io.Writer { return p.out }
func (p *printer) Err() io.Writer { return p.err }
