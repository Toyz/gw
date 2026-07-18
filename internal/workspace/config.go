package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// DefaultConfigName is the filename `gw config init` scaffolds — the preferred,
// first-checked candidate.
const DefaultConfigName = "gw.toml"

// configCandidates are the config filenames LoadConfig looks for in the root,
// in priority order. TOML is preferred; YAML is still accepted. The first one
// that exists wins, so a repo with both gw.toml and gw.yaml uses gw.toml.
var configCandidates = []string{"gw.toml", "gw.yaml", "gw.yml"}

// Config is the optional per-workspace config (gw.toml, or gw.yaml). Every field
// is optional; a missing file yields the zero value plus the built-in defaults
// applied by LoadConfig.
type Config struct {
	// Root overrides the workspace root (relative to the file, or absolute).
	Root string `toml:"root" yaml:"root"`
	// Ignore is a list of path globs (matched against the path relative to the
	// root, using filepath.Match semantics per segment) to skip during discovery.
	Ignore []string `toml:"ignore" yaml:"ignore"`
	// Pins forces specific dependency versions when running `lint --fix`.
	// Keyed by module path, e.g. "github.com/foo/bar": "v1.4.0".
	Pins map[string]string `toml:"pins" yaml:"pins"`
	// Env holds environment variables applied to every command gw runs across the
	// workspace (run/test/tidy, hooks, and extension commands). Opt-in: nothing is
	// injected unless it is declared here or via EnvFiles / the --env* flags.
	Env map[string]string `toml:"env" yaml:"env"`
	// EnvFiles lists dotenv files (relative to the root, or absolute) layered on
	// top of Env, in order. Later files and inline Env keys lose to CLI --env.
	EnvFiles []string `toml:"env_files" yaml:"envFiles"`
	// Commands declares custom `gw <name>` subcommands natively — no compiled
	// .gw/build.go needed. Keyed by command name: [commands.<name>].
	Commands map[string]ConfigCommand `toml:"commands" yaml:"commands"`
	// Hooks declares lifecycle hooks natively, keyed by event: [hooks.<event>]
	// (e.g. [hooks.pre-build]). Same shape as Commands.
	Hooks map[string]ConfigCommand `toml:"hooks" yaml:"hooks"`
	// Projects declares non-Go units in the repo, keyed by name: [projects.<name>].
	// A Go module is a build unit gw auto-discovers; a project is a unit in another
	// language (Rust, Python, ...) gw can't infer. gw's verbs (build/test/vet/...)
	// dispatch to each unit's toolchain, and `gw affected --projects` maps a diff to
	// the projects it touches (by directory). Go modules are NOT listed here.
	Projects map[string]Project `toml:"projects" yaml:"projects"`
	// Toolchains defines how a toolchain runs each verb: name -> verb -> shell
	// command (run via `sh -c` in the unit's directory). Built-in `go` and `rust`
	// are first-party; entries here add new languages OR override the built-ins
	// (e.g. [toolchains.rust] test = "cargo nextest run").
	Toolchains map[string]map[string]string `toml:"toolchains" yaml:"toolchains"`
}

// Project is a non-Go unit declared under [projects.<name>]. Path is its
// directory (defaults to the name); Toolchain selects how verbs run (a built-in
// `go`/`rust` or a [toolchains.<name>] entry); Tasks overrides individual verbs.
type Project struct {
	Path      string            `toml:"path" yaml:"path"`
	Toolchain string            `toml:"toolchain" yaml:"toolchain"`
	Tasks     map[string]string `toml:"tasks" yaml:"tasks"` // verb -> shell command override
}

// ConfigCommand is a command or hook declared in gw.toml/gw.yaml: an ordered
// list of steps run natively by gw. Shared by [commands.<name>] and
// [hooks.<event>].
type ConfigCommand struct {
	// Desc is the one-line help shown in `gw --help` and `gw list` (commands only).
	Desc string `toml:"desc" yaml:"desc"`
	// Steps run in order. A step "<module>:<verb>" (verb in build/test/vet/
	// generate/tidy/run, no spaces) runs that go command in the module's
	// directory; any other string is a shell command (sh -c) run in Dir.
	Steps []string `toml:"steps" yaml:"steps"`
	// Dir sets the working directory for shell steps: a module (by path or name),
	// else a path relative to the root. Module:verb steps ignore it.
	Dir string `toml:"dir" yaml:"dir"`
	// Args names the positional CLI args a command takes (commands only, not hooks):
	// args = ["service"] makes `gw <cmd> <service>` bind $service (and $1) in steps.
	// Empty means the command takes arbitrary positional args ($1, $2, $@).
	Args []string `toml:"args" yaml:"args"`
}

// Empty reports whether the command declares no work (nothing to run).
func (c ConfigCommand) Empty() bool { return len(c.Steps) == 0 }

// defaultIgnores are directory names never walked into during discovery. ".gw"
// holds gw's own compiled extension module (gwext.local); it is intentionally
// not a workspace member, so it must never land in go.work or lint/graph output.
var defaultIgnores = []string{".git", ".gw", "vendor", "testdata", "node_modules", ".idea", ".vscode"}

// LoadConfig reads the first of configCandidates that exists in root. A missing
// file is not an error: it returns a zero Config. Parse/read errors are returned,
// tagged with the offending filename.
func LoadConfig(root string) (Config, error) {
	var cfg Config
	for _, name := range configCandidates {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return cfg, err
		}
		if strings.HasSuffix(name, ".toml") {
			if err := toml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("%s: %w", name, err)
			}
		} else if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("%s: %w", name, err)
		}
		return cfg, nil
	}
	return cfg, nil
}

// ConfigPath returns the path of the config file gw would load for root — the
// first of configCandidates that exists — and whether one was found.
func ConfigPath(root string) (string, bool) {
	for _, name := range configCandidates {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}
