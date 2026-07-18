package workspace

import (
	"path/filepath"
	"sort"
)

// Unit is a runnable member of the workspace: a discovered Go module (toolchain
// "go") or a declared [projects.<name>]. gw's verbs and `<unit>:<verb>` steps
// operate on units; TaskCommand resolves each unit+verb to a command.
type Unit struct {
	Name      string
	Dir       string            // absolute
	Toolchain string            // "go" for modules; the project's toolchain otherwise
	Tasks     map[string]string // per-verb override (projects only)
	IsModule  bool              // a discovered Go module (vs a declared project)
}

// Units returns every runnable unit — Go modules (toolchain "go") plus declared
// projects — sorted by directory. root is the workspace root that project paths
// are relative to. overlaps names any project whose directory collides with a Go
// module's (the module wins; the caller may warn). Projects without a toolchain
// default to "go".
func Units(root string, mods []Module, projects map[string]Project) (units []Unit, overlaps []string) {
	byDir := make(map[string]bool, len(mods))
	units = make([]Unit, 0, len(mods)+len(projects))
	for _, m := range mods {
		units = append(units, Unit{Name: m.Path, Dir: m.Dir, Toolchain: "go", IsModule: true})
		byDir[filepath.Clean(m.Dir)] = true
	}
	names := make([]string, 0, len(projects))
	for n := range projects {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := projects[name]
		path := p.Path
		if path == "" {
			path = name
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		dir := filepath.Clean(path)
		if byDir[dir] {
			overlaps = append(overlaps, name)
			continue // module wins
		}
		tc := p.Toolchain
		if tc == "" {
			tc = "go"
		}
		units = append(units, Unit{Name: name, Dir: dir, Toolchain: tc, Tasks: p.Tasks})
	}
	sort.Slice(units, func(i, j int) bool { return units[i].Dir < units[j].Dir })
	return units, overlaps
}
