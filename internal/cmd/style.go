package cmd

import (
	"io"
	"os"
)

// ANSI SGR codes for the small palette gw uses.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// styler paints strings with ANSI color when the target stream supports it. The
// zero value (on == false) is a no-op, so plain output is the safe default.
type styler struct{ on bool }

// newStyler enables color only for a real terminal with color not disabled — so
// piped output, --json, and CI logs stay plain automatically.
func newStyler(w io.Writer) styler { return styler{on: colorEnabled(w)} }

func (s styler) wrap(code, v string) string {
	if !s.on || v == "" {
		return v
	}
	return code + v + ansiReset
}

func (s styler) red(v string) string    { return s.wrap(ansiRed, v) }
func (s styler) green(v string) string  { return s.wrap(ansiGreen, v) }
func (s styler) yellow(v string) string { return s.wrap(ansiYellow, v) }
func (s styler) cyan(v string) string   { return s.wrap(ansiCyan, v) }
func (s styler) dim(v string) string    { return s.wrap(ansiDim, v) }
func (s styler) bold(v string) string   { return s.wrap(ansiBold, v) }

// colorEnabled reports whether ANSI color should be written to w: only for a
// real terminal, with NO_COLOR unset and TERM not "dumb". No dependency — it
// checks the file's character-device mode directly.
func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
