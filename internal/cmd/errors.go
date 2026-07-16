package cmd

import "fmt"

// cmdError is a user-facing failure with an optional actionable hint. The
// top-level handler renders it as "✗ <msg>" and, when a hint is set, a dim
// "help: <hint>" line — the way half of gw's errors have an obvious next step.
type cmdError struct {
	msg  string
	hint string
}

func (e *cmdError) Error() string { return e.msg }

// failf builds a cmdError from a printf-style message (no hint).
func failf(format string, a ...any) *cmdError {
	return &cmdError{msg: fmt.Sprintf(format, a...)}
}

// withHint attaches an actionable next step (e.g. "run `gw sync`") and returns
// the error for chaining: `return failf("...").withHint("run `gw init`")`.
func (e *cmdError) withHint(format string, a ...any) *cmdError {
	e.hint = fmt.Sprintf(format, a...)
	return e
}
