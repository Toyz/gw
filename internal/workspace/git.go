package workspace

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// GitRoot returns the top-level git directory containing dir.
func GitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ChangedFiles returns absolute paths of files that differ between ref and the
// current working tree (tracked changes, staged and unstaged). gitRoot is the
// repository top level. Empty/whitespace lines are skipped.
func ChangedFiles(gitRoot, ref string) ([]string, error) {
	cmd := exec.Command("git", "-C", gitRoot, "diff", "--name-only", ref)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, filepath.Join(gitRoot, filepath.FromSlash(line)))
	}
	return files, nil
}

// OwningModule returns the workspace module that owns absFile: the member whose
// directory is the longest prefix of the file's path. ok is false if none owns it.
func OwningModule(mods []Module, absFile string) (owner Module, ok bool) {
	best := -1
	for _, m := range mods {
		if absFile == m.Dir || strings.HasPrefix(absFile, m.Dir+string(filepath.Separator)) {
			if len(m.Dir) > best {
				best = len(m.Dir)
				owner = m
				ok = true
			}
		}
	}
	return owner, ok
}

// AffectedModules maps changed files to owning modules (seeds) and returns the
// transitive set of impacted module paths (seeds plus everything depending on
// them), sorted. Files owned by no module are ignored.
func AffectedModules(g *Graph, mods []Module, changed []string) (seeds, impacted []string) {
	seedSet := map[string]bool{}
	for _, f := range changed {
		if m, ok := OwningModule(mods, f); ok {
			seedSet[m.Path] = true
		}
	}
	for p := range seedSet {
		seeds = append(seeds, p)
	}
	sort.Strings(seeds)
	return seeds, g.TransitiveDependents(seeds)
}
