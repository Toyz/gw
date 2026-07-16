package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// WorkFileName is the workspace file name.
const WorkFileName = "go.work"

// workFilePath returns the go.work path inside root.
func workFilePath(root string) string { return filepath.Join(root, WorkFileName) }

// ReadWorkFile parses root's go.work. It returns (nil, nil) if none exists.
func ReadWorkFile(root string) (*modfile.WorkFile, error) {
	data, err := os.ReadFile(workFilePath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return modfile.ParseWork(WorkFileName, data, nil)
}

// WorkFileExists reports whether root already has a go.work.
func WorkFileExists(root string) bool {
	_, err := os.Stat(workFilePath(root))
	return err == nil
}

// UsePath returns the go.work `use` path for a module dir relative to root,
// always slash-separated and prefixed with "./" (or "." for the root itself).
func UsePath(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		rel = dir
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "."
	}
	return "./" + rel
}

// SetUseSet rewrites wf's `use` directives to exactly the given modules,
// preserving all other blocks (go, toolchain, replace, godebug). It returns the
// use paths added and removed relative to wf's previous state, both sorted.
func SetUseSet(wf *modfile.WorkFile, root string, mods []Module) (added, removed []string) {
	want := make(map[string]string, len(mods))
	for _, m := range mods {
		want[UsePath(root, m.Dir)] = m.Path
	}
	have := make(map[string]bool, len(wf.Use))
	var existing []string
	for _, u := range wf.Use {
		if u.Path == "" {
			continue
		}
		if !have[u.Path] {
			existing = append(existing, u.Path)
		}
		have[u.Path] = true
	}
	for p := range want {
		if !have[p] {
			added = append(added, p)
		}
	}
	for p := range have {
		if _, ok := want[p]; !ok {
			removed = append(removed, p)
		}
	}

	// modfile.SetUse re-adds every wanted path unconditionally, duplicating any
	// already present. Drop all existing use entries first, then add the wanted
	// set exactly once, sorted for stable output.
	for _, p := range existing {
		_ = wf.DropUse(p)
	}
	paths := make([]string, 0, len(want))
	for p := range want {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		wf.AddNewUse(p, want[p])
	}

	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// NewWorkFile builds an empty go.work with a go directive derived from the
// modules (highest go version present) or the running toolchain as a fallback.
func NewWorkFile(mods []Module) (*modfile.WorkFile, error) {
	wf, err := modfile.ParseWork(WorkFileName, []byte{}, nil)
	if err != nil {
		return nil, err
	}
	if err := wf.AddGoStmt(pickGoVersion(mods)); err != nil {
		return nil, err
	}
	return wf, nil
}

// pickGoVersion returns the highest `go` directive across mods, or the running
// toolchain's version (e.g. "1.26.0") when none declare one.
func pickGoVersion(mods []Module) string {
	best := ""
	for _, m := range mods {
		if m.GoVersion == "" {
			continue
		}
		if best == "" || semver.Compare("v"+m.GoVersion, "v"+best) > 0 {
			best = m.GoVersion
		}
	}
	if best != "" {
		return best
	}
	return UserGoVersion()
}

// UserGoVersion returns the user's `go` toolchain version without the "go"
// prefix (e.g. "1.25.0"), read from `go env GOVERSION` — so files gw generates
// (go.work, the scaffolded .gw/go.mod) match the Go the user actually runs, not
// the version gw itself was built with. Falls back to gw's own build version if
// `go` can't be queried.
func UserGoVersion() string {
	if out, err := exec.Command("go", "env", "GOVERSION").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return strings.TrimPrefix(v, "go")
		}
	}
	return strings.TrimPrefix(runtime.Version(), "go")
}

// WriteWorkFile formats and writes wf to root's go.work.
func WriteWorkFile(root string, wf *modfile.WorkFile) error {
	wf.Cleanup()
	return os.WriteFile(workFilePath(root), modfile.Format(wf.Syntax), 0o644)
}

// FormatWorkFile returns the formatted go.work bytes (for --dry-run previews).
func FormatWorkFile(wf *modfile.WorkFile) []byte {
	wf.Cleanup()
	return modfile.Format(wf.Syntax)
}

// HoistReplaces moves every `replace` directive out of each module's go.mod and
// into wf. Filesystem replace targets (relative local paths) are rewritten to be
// relative to root. It mutates wf and the modules' GoMod in memory and returns
// the modules whose go.mod changed plus human-readable conflict warnings. The
// caller is responsible for persisting wf and each returned module.
func HoistReplaces(root string, wf *modfile.WorkFile, mods []Module) (mutated []Module, warnings []string) {
	// Track the target chosen for each (oldPath, oldVers) to detect conflicts.
	type key struct{ path, vers string }
	chosen := map[key]module.Version{}

	for i := range mods {
		m := &mods[i]
		if len(m.GoMod.Replace) == 0 {
			continue
		}
		changed := false
		// Copy slice: DropReplace mutates m.GoMod.Replace underneath us.
		reps := make([]*modfile.Replace, len(m.GoMod.Replace))
		copy(reps, m.GoMod.Replace)
		for _, r := range reps {
			newPath, newVers := r.New.Path, r.New.Version
			if isLocalPath(newPath) {
				newPath = rebaseLocal(root, m.Dir, newPath)
			}
			k := key{r.Old.Path, r.Old.Version}
			if prev, ok := chosen[k]; ok {
				if prev.Path != newPath || prev.Version != newVers {
					warnings = append(warnings, fmt.Sprintf(
						"conflicting replace for %s: keeping %s, ignoring %s (in %s)",
						replaceLHS(r), fmtTarget(prev.Path, prev.Version),
						fmtTarget(newPath, newVers), m.Path))
				}
				// Still drop it from this module so the workspace owns it.
				_ = m.GoMod.DropReplace(r.Old.Path, r.Old.Version)
				changed = true
				continue
			}
			chosen[k] = module.Version{Path: newPath, Version: newVers}
			if err := wf.AddReplace(r.Old.Path, r.Old.Version, newPath, newVers); err != nil {
				warnings = append(warnings, fmt.Sprintf("could not hoist replace %s: %v", replaceLHS(r), err))
				continue
			}
			_ = m.GoMod.DropReplace(r.Old.Path, r.Old.Version)
			changed = true
		}
		if changed {
			mutated = append(mutated, *m)
		}
	}
	return mutated, warnings
}

func replaceLHS(r *modfile.Replace) string {
	if r.Old.Version == "" {
		return r.Old.Path
	}
	return r.Old.Path + "@" + r.Old.Version
}

func fmtTarget(path, vers string) string {
	if vers == "" {
		return path
	}
	return path + "@" + vers
}

// isLocalPath reports whether a replace target is a filesystem path.
func isLocalPath(p string) bool {
	return p == "." || p == ".." ||
		strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") ||
		strings.HasPrefix(p, "/") || filepath.IsAbs(p)
}

// rebaseLocal rewrites a local replace target that is relative to modDir so it
// becomes relative to root (as go.work expects). Absolute paths are returned
// unchanged.
func rebaseLocal(root, modDir, target string) string {
	if filepath.IsAbs(target) {
		return filepath.ToSlash(target)
	}
	abs := filepath.Join(modDir, target)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(target)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}
