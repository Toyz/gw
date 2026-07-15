package workspace

import (
	"os"

	"golang.org/x/mod/modfile"
)

// Module is a single Go module discovered inside the workspace.
type Module struct {
	// Path is the module path (from the `module` directive), e.g. github.com/foo/svc-a.
	Path string
	// Dir is the absolute directory containing the module's go.mod.
	Dir string
	// GoMod is the parsed go.mod, kept so callers can mutate + rewrite it.
	GoMod *modfile.File
	// Requires maps required module path -> version, excluding indirect requires.
	Requires map[string]string
	// GoVersion is the `go` directive value (e.g. "1.25.0"), empty if absent.
	GoVersion string
	// Toolchain is the `toolchain` directive value (e.g. "go1.26.0"), empty if absent.
	Toolchain string
}

// loadModule parses the go.mod at goModPath and builds a Module. dir is the
// absolute directory containing that go.mod.
func loadModule(goModPath, dir string) (Module, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return Module{}, err
	}
	mf, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return Module{}, err
	}

	m := Module{Dir: dir, GoMod: mf, Requires: map[string]string{}}
	if mf.Module != nil {
		m.Path = mf.Module.Mod.Path
	}
	if mf.Go != nil {
		m.GoVersion = mf.Go.Version
	}
	if mf.Toolchain != nil {
		m.Toolchain = mf.Toolchain.Name
	}
	for _, r := range mf.Require {
		if r.Indirect {
			continue
		}
		m.Requires[r.Mod.Path] = r.Mod.Version
	}
	return m, nil
}

// Save formats and writes the module's (possibly mutated) go.mod back to disk.
func (m Module) Save() error {
	m.GoMod.Cleanup()
	out := modfile.Format(m.GoMod.Syntax)
	return os.WriteFile(goModPath(m.Dir), out, 0o644)
}
