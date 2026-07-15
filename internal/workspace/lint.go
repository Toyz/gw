package workspace

import (
	"sort"

	"golang.org/x/mod/semver"
)

// Synthetic dependency keys used for directive mismatches.
const (
	GoDirective        = "go"
	ToolchainDirective = "toolchain"
)

// Mismatch is a dependency (or directive) required at more than one version
// across the workspace.
type Mismatch struct {
	// Dep is the module path, or the synthetic key GoDirective/ToolchainDirective.
	Dep string
	// Versions maps each distinct version -> the module paths declaring it, sorted.
	Versions map[string][]string
}

// SortedVersions returns the distinct versions of a Mismatch, highest first.
func (m Mismatch) SortedVersions() []string {
	vs := make([]string, 0, len(m.Versions))
	for v := range m.Versions {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return compareVer(vs[i], vs[j]) > 0 })
	return vs
}

// Lint reports dependency and directive version mismatches across the workspace.
// Requires that point at another workspace member are ignored (the `use`
// directive resolves those, so their version is irrelevant).
func Lint(mods []Module) []Mismatch {
	inWorkspace := make(map[string]bool, len(mods))
	for _, m := range mods {
		inWorkspace[m.Path] = true
	}

	// dep -> version -> set of module paths.
	seen := map[string]map[string]map[string]bool{}
	record := func(dep, version, mod string) {
		if version == "" {
			return
		}
		if seen[dep] == nil {
			seen[dep] = map[string]map[string]bool{}
		}
		if seen[dep][version] == nil {
			seen[dep][version] = map[string]bool{}
		}
		seen[dep][version][mod] = true
	}

	for _, m := range mods {
		for dep, ver := range m.Requires {
			if inWorkspace[dep] {
				continue
			}
			record(dep, ver, m.Path)
		}
		record(GoDirective, m.GoVersion, m.Path)
		record(ToolchainDirective, m.Toolchain, m.Path)
	}

	var out []Mismatch
	for dep, versions := range seen {
		if len(versions) < 2 {
			continue
		}
		vm := make(map[string][]string, len(versions))
		for v, modset := range versions {
			mods := make([]string, 0, len(modset))
			for mp := range modset {
				mods = append(mods, mp)
			}
			sort.Strings(mods)
			vm[v] = mods
		}
		out = append(out, Mismatch{Dep: dep, Versions: vm})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dep < out[j].Dep })
	return out
}

// Strategy selects which version wins when aligning a mismatch.
type Strategy string

const (
	Highest Strategy = "highest"
	Lowest  Strategy = "lowest"
)

// Fix aligns dependency version mismatches by rewriting each affected module's
// go.mod. Directive mismatches (go/toolchain) are never auto-rewritten. pins
// (from gw.yaml) override the strategy for a given dependency. It returns the
// modules whose go.mod changed; the caller persists them via Module.Save.
func Fix(mods []Module, mismatches []Mismatch, strategy Strategy, pins map[string]string) []Module {
	target := map[string]string{}
	for _, mm := range mismatches {
		if mm.Dep == GoDirective || mm.Dep == ToolchainDirective {
			continue
		}
		if pinned, ok := pins[mm.Dep]; ok {
			target[mm.Dep] = pinned
			continue
		}
		target[mm.Dep] = pickVersion(mm.SortedVersions(), strategy)
	}

	changedSet := map[string]*Module{}
	for i := range mods {
		m := &mods[i]
		for dep, want := range target {
			cur, required := m.Requires[dep]
			if !required || cur == want {
				continue
			}
			if err := m.GoMod.AddRequire(dep, want); err != nil {
				continue
			}
			m.Requires[dep] = want
			changedSet[m.Dir] = m
		}
	}

	// Preserve deterministic order by module dir.
	dirs := make([]string, 0, len(changedSet))
	for d := range changedSet {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	out := make([]Module, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, *changedSet[d])
	}
	return out
}

// pickVersion returns the winning version for a sorted-desc list per strategy.
func pickVersion(sortedDesc []string, strategy Strategy) string {
	if len(sortedDesc) == 0 {
		return ""
	}
	if strategy == Lowest {
		return sortedDesc[len(sortedDesc)-1]
	}
	return sortedDesc[0]
}

// compareVer compares two version strings. Valid semver is compared with
// semver.Compare; otherwise it falls back to lexical comparison so
// non-canonical directive values (like "1.25.0") still order deterministically.
func compareVer(a, b string) int {
	as, bs := "v"+a, "v"+b
	if semver.IsValid(as) && semver.IsValid(bs) {
		return semver.Compare(as, bs)
	}
	if semver.IsValid(a) && semver.IsValid(b) {
		return semver.Compare(a, b)
	}
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
