package workspace

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

// gitOutput runs `git -C dir <args...>` and returns its raw stdout.
func gitOutput(dir string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
}

// gitLines runs git and returns stdout split into trimmed, non-empty lines.
func gitLines(dir string, args ...string) ([]string, error) {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, ln := range strings.Split(string(out), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			lines = append(lines, ln)
		}
	}
	return lines, nil
}

// gitOK runs git purely for its exit status (output discarded); true on exit 0.
func gitOK(dir string, args ...string) bool {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Run() == nil
}

// relSlash returns target relative to base, slash-separated ("." for the same
// directory). It errors only when no relative path exists (e.g. other volume).
func relSlash(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// GitRoot returns the top-level git directory containing dir.
func GitRoot(dir string) (string, error) {
	out, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ChangedFiles returns absolute paths of files that differ between ref and the
// current working tree (tracked changes, staged and unstaged). gitRoot is the
// repository top level.
func ChangedFiles(gitRoot, ref string) ([]string, error) {
	lines, err := gitLines(gitRoot, "diff", "--name-only", ref)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(lines))
	for _, line := range lines {
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

// AffectedServices maps changed files to the declared services that own them,
// by directory (longest-prefix wins, like OwningModule). root is the workspace
// root that service paths are relative to; changed are absolute file paths.
// Returns the affected service names, sorted. Services with no path default to
// their name as the directory.
func AffectedServices(root string, services map[string]Service, changed []string) []string {
	if len(services) == 0 {
		return nil
	}
	type svc struct{ name, dir string }
	dirs := make([]svc, 0, len(services))
	for name, s := range services {
		p := s.Path
		if p == "" {
			p = name
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		dirs = append(dirs, svc{name, filepath.Clean(p)})
	}

	hit := map[string]bool{}
	for _, f := range changed {
		best, name := -1, ""
		for _, s := range dirs {
			if f == s.dir || strings.HasPrefix(f, s.dir+string(filepath.Separator)) {
				if len(s.dir) > best {
					best, name = len(s.dir), s.name
				}
			}
		}
		if name != "" {
			hit[name] = true
		}
	}
	out := make([]string, 0, len(hit))
	for n := range hit {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ModuleTagPrefix returns the git version-tag prefix for a module directory,
// per Go's multi-module convention: a module in sub-directory "svc/api" tags as
// "svc/api/vX.Y.Z", while a module at the repo root tags as "vX.Y.Z" (empty
// prefix). gitRoot is the repository top level; modDir is absolute.
func ModuleTagPrefix(gitRoot, modDir string) string {
	rel, err := relSlash(gitRoot, modDir)
	if err != nil || rel == "." || rel == "" {
		return ""
	}
	return rel + "/"
}

// ModuleTags returns the semver versions published for the module at modDir
// (the part after the tag prefix, e.g. "v1.2.0"), sorted newest first. Tags that
// aren't valid semver under the module's prefix are ignored.
func ModuleTags(gitRoot, modDir string) ([]string, error) {
	prefix := ModuleTagPrefix(gitRoot, modDir)
	tags, err := gitLines(gitRoot, "tag", "-l", prefix+"v*")
	if err != nil {
		return nil, err
	}
	var vers []string
	for _, tag := range tags {
		rest := strings.TrimPrefix(tag, prefix)
		// For the root module (empty prefix), a tag containing "/" belongs to a
		// sub-module, not the root.
		if prefix == "" && strings.Contains(rest, "/") {
			continue
		}
		if !semver.IsValid(rest) {
			continue
		}
		vers = append(vers, rest)
	}
	sort.Slice(vers, func(i, j int) bool { return semver.Compare(vers[i], vers[j]) > 0 })
	return vers, nil
}

// TagExists reports whether an exact tag ref exists in the repository.
func TagExists(gitRoot, tag string) bool {
	return gitOK(gitRoot, "show-ref", "--verify", "--quiet", "refs/tags/"+tag)
}

// SubtreeChanged reports whether module m's own files differ between ref (a tag
// or commit) and the working tree. "Own files" excludes any nested workspace
// member's directory: a change inside a sub-module counts against that
// sub-module, not m. all is the full member set (for ownership resolution).
func SubtreeChanged(gitRoot, ref string, m Module, all []Module) (bool, error) {
	rel, err := relSlash(gitRoot, m.Dir)
	if err != nil {
		return false, err
	}
	if rel == "" {
		rel = "."
	}
	lines, err := gitLines(gitRoot, "diff", "--name-only", ref, "--", rel)
	if err != nil {
		return false, err
	}
	for _, line := range lines {
		abs := filepath.Join(gitRoot, filepath.FromSlash(line))
		if owner, ok := OwningModule(all, abs); ok && owner.Dir == m.Dir {
			return true, nil
		}
	}
	return false, nil
}
