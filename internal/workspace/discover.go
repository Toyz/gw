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

// matchGlob matches a slash-separated relative path against a glob pattern,
// segment by segment. "**" matches zero or more whole path segments; within a
// segment, filepath.Match syntax (*, ?, [..]) applies. A slashless pattern also
// matches any single path segment (so "testdata" skips it at any depth), but a
// pattern like "**/testdata" matches only a segment named exactly "testdata" —
// never "testdata-fixtures".
func matchGlob(pattern, rel string) bool {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)
	if matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/")) {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := filepath.Match(pattern, filepath.Base(rel)); ok {
			return true
		}
	}
	return false
}

// matchSegments reports whether the path segments name match the pattern
// segments pat, treating "**" as zero-or-more segments.
func matchSegments(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		// Collapse consecutive ** and try consuming 0..len(name) segments.
		for i := 0; i <= len(name); i++ {
			if matchSegments(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	if ok, err := filepath.Match(pat[0], name[0]); err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], name[1:])
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
