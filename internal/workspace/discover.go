package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// goModPath returns the path to the go.mod inside dir.
func goModPath(dir string) string { return filepath.Join(dir, "go.mod") }

// Discover walks root and returns every Go module found, sorted by directory.
// Directories in defaultIgnores (and anything matching cfg.Ignore globs) are
// skipped. Nested modules are returned as separate entries.
func Discover(root string, cfg Config) ([]Module, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var mods []Module
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Never descend into the root itself as a skip candidate.
			if path != root && shouldSkipDir(root, path, d.Name(), cfg) {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		m, lerr := loadModule(path, filepath.Dir(path))
		if lerr != nil {
			return lerr
		}
		mods = append(mods, m)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(mods, func(i, j int) bool { return mods[i].Dir < mods[j].Dir })
	return mods, nil
}

// shouldSkipDir reports whether a directory should be skipped during discovery.
func shouldSkipDir(root, path, name string, cfg Config) bool {
	for _, ig := range defaultIgnores {
		if name == ig {
			return true
		}
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, pat := range cfg.Ignore {
		if matchGlob(pat, rel) {
			return true
		}
	}
	return false
}

// matchGlob matches a slash-separated path against a glob pattern. It supports a
// trailing (or embedded) "**" to match across path segments; otherwise it falls
// back to path.Match-style per-string matching on the full relative path.
func matchGlob(pattern, rel string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.Contains(pattern, "**") {
		// Split on "**" and require the literal parts to appear in order.
		parts := strings.Split(pattern, "**")
		idx := 0
		for i, p := range parts {
			p = strings.Trim(p, "/")
			if p == "" {
				continue
			}
			j := strings.Index(rel[idx:], p)
			if j < 0 {
				return false
			}
			// First part must anchor at the start.
			if i == 0 && j != 0 {
				return false
			}
			idx += j + len(p)
		}
		return true
	}
	ok, _ := filepath.Match(pattern, rel)
	if ok {
		return true
	}
	// Also match against the final segment for convenience (e.g. "testdata").
	ok, _ = filepath.Match(pattern, filepath.Base(rel))
	return ok
}

// FindRoot searches upward from start for a directory containing go.work.
// If none is found it returns start unchanged (and ok=false).
func FindRoot(start string) (root string, ok bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return start, false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start, false
		}
		dir = parent
	}
}
