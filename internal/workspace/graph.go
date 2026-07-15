package workspace

import (
	"path/filepath"
	"sort"
)

// Graph is the intra-workspace module dependency DAG. An edge A->B means module
// A requires module B and B is also a workspace member.
type Graph struct {
	// Modules are the workspace members, sorted by path.
	Modules []Module
	byPath  map[string]Module
	// deps maps a module path to the workspace module paths it requires.
	deps map[string][]string
	// dependents maps a module path to the workspace module paths requiring it.
	dependents map[string][]string
}

// BuildGraph constructs the dependency DAG from discovered modules.
func BuildGraph(mods []Module) *Graph {
	g := &Graph{
		byPath:     make(map[string]Module, len(mods)),
		deps:       map[string][]string{},
		dependents: map[string][]string{},
	}
	member := make(map[string]bool, len(mods))
	byDir := make(map[string]string, len(mods)) // abs dir -> module path
	for _, m := range mods {
		g.byPath[m.Path] = m
		member[m.Path] = true
		if m.Dir != "" {
			byDir[filepath.Clean(m.Dir)] = m.Path
		}
	}
	for _, m := range mods {
		edges := map[string]bool{}
		// Direct requires.
		for dep := range m.Requires {
			if member[dep] {
				edges[dep] = true
			}
		}
		if m.GoMod != nil {
			// Indirect requires that still point at a workspace member.
			for _, r := range m.GoMod.Require {
				if member[r.Mod.Path] {
					edges[r.Mod.Path] = true
				}
			}
			// Local replace targets (=> ../foo) resolving to a member directory.
			for _, r := range m.GoMod.Replace {
				if !isLocalPath(r.New.Path) {
					continue
				}
				target := r.New.Path
				if !filepath.IsAbs(target) {
					target = filepath.Join(m.Dir, target)
				}
				if dep, ok := byDir[filepath.Clean(target)]; ok {
					edges[dep] = true
				}
			}
		}
		for dep := range edges {
			if dep == m.Path {
				continue
			}
			g.deps[m.Path] = append(g.deps[m.Path], dep)
			g.dependents[dep] = append(g.dependents[dep], m.Path)
		}
	}
	for k := range g.deps {
		sort.Strings(g.deps[k])
	}
	for k := range g.dependents {
		sort.Strings(g.dependents[k])
	}

	g.Modules = append(g.Modules, mods...)
	sort.Slice(g.Modules, func(i, j int) bool { return g.Modules[i].Path < g.Modules[j].Path })
	return g
}

// Has reports whether path is a workspace member.
func (g *Graph) Has(path string) bool { _, ok := g.byPath[path]; return ok }

// Module returns the member with the given path (zero Module if absent).
func (g *Graph) Module(path string) Module { return g.byPath[path] }

// Dependencies returns the direct workspace dependencies of path, sorted.
func (g *Graph) Dependencies(path string) []string { return g.deps[path] }

// Dependents returns the direct workspace dependents of path, sorted.
func (g *Graph) Dependents(path string) []string { return g.dependents[path] }

// TransitiveDependents returns seeds plus every module that transitively depends
// on any seed (i.e. everything that must be rebuilt/retested when seeds change),
// sorted and de-duplicated.
func (g *Graph) TransitiveDependents(seeds []string) []string {
	seen := map[string]bool{}
	var queue []string
	for _, s := range seeds {
		if g.Has(s) && !seen[s] {
			seen[s] = true
			queue = append(queue, s)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range g.dependents[cur] {
			if !seen[d] {
				seen[d] = true
				queue = append(queue, d)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Edges returns every A->B dependency edge, sorted by (from, to).
func (g *Graph) Edges() [][2]string {
	var edges [][2]string
	for from, tos := range g.deps {
		for _, to := range tos {
			edges = append(edges, [2]string{from, to})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}
		return edges[i][1] < edges[j][1]
	})
	return edges
}
