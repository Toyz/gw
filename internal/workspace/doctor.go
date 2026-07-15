package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Severity ranks a diagnostic.
type Severity string

const (
	SevError Severity = "error"
	SevWarn  Severity = "warn"
	SevInfo  Severity = "info"
)

// Issue is a single workspace-health finding with an actionable fix hint.
type Issue struct {
	Severity Severity
	Msg      string
	Fix      string
}

// Diagnose inspects workspace health and returns findings: a missing or stale
// go.work, orphaned use directories, un-hoisted replace directives, and version
// mismatches. mods is the discovered module set; cfg is the loaded config.
func Diagnose(root string, cfg Config, mods []Module) []Issue {
	var issues []Issue
	add := func(s Severity, msg, fix string) { issues = append(issues, Issue{s, msg, fix}) }

	wf, err := ReadWorkFile(root)
	if err != nil {
		add(SevError, fmt.Sprintf("go.work is unparseable: %v", err), "fix or delete go.work, then `gw init`")
		return issues
	}
	if wf == nil {
		add(SevError, "no go.work in workspace root", "run `gw init`")
		return issues
	}

	// Compare the go.work use set against discovered modules.
	want := map[string]bool{}
	for _, m := range mods {
		want[UsePath(root, m.Dir)] = true
	}
	have := map[string]bool{}
	for _, u := range wf.Use {
		if u.Path == "" {
			continue
		}
		have[u.Path] = true
		// Orphaned: a use entry whose directory no longer holds a go.mod.
		dir := filepath.Join(root, filepath.FromSlash(u.Path))
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr != nil {
			add(SevError, fmt.Sprintf("use %s has no go.mod", u.Path), "run `gw sync` or `gw remove "+u.Path+"`")
		} else if !want[u.Path] {
			add(SevWarn, fmt.Sprintf("use %s is not discovered (ignored by gw.yaml?)", u.Path),
				"adjust gw.yaml ignore globs or `gw remove "+u.Path+"`")
		}
	}
	for _, m := range mods {
		if !have[UsePath(root, m.Dir)] {
			add(SevWarn, fmt.Sprintf("module %s (%s) is missing from go.work", m.Path, UsePath(root, m.Dir)),
				"run `gw sync`")
		}
	}

	// Un-hoisted replace directives still living in module go.mod files.
	for _, m := range mods {
		if n := len(m.GoMod.Replace); n > 0 {
			add(SevInfo, fmt.Sprintf("module %s has %d replace directive(s) in its go.mod", m.Path, n),
				"run `gw init --force` to hoist them into go.work")
		}
	}

	// Version + directive mismatches.
	for _, mm := range Lint(mods) {
		switch mm.Dep {
		case GoDirective, ToolchainDirective:
			add(SevWarn, fmt.Sprintf("%s directive differs across modules: %v", mm.Dep, mm.SortedVersions()),
				"align the "+mm.Dep+" directive manually")
		default:
			add(SevWarn, fmt.Sprintf("%s required at %d versions: %v", mm.Dep, len(mm.Versions), mm.SortedVersions()),
				"run `gw lint --fix`")
		}
	}

	return issues
}

// CountBySeverity returns how many issues carry each severity.
func CountBySeverity(issues []Issue) (errors, warns, infos int) {
	for _, i := range issues {
		switch i.Severity {
		case SevError:
			errors++
		case SevWarn:
			warns++
		case SevInfo:
			infos++
		}
	}
	return
}
