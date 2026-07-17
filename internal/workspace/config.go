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
	// Services declares deployable units in the repo, keyed by service name:
	// [services.<name>]. Unlike a Go module (a build unit), a service is a
	// runnable unit — and it need not be Go at all (e.g. a Rust binary). gw's Go
	// features (workspace/lint) ignore services; `gw affected` maps a diff to the
	// services it touches (by directory), so a polyglot monorepo gets
	// change-based redeploy across languages.
	Services map[string]Service `toml:"services" yaml:"services"`
}

// Service is a deployable unit declared in gw.toml/gw.yaml under
// [services.<name>]. gw core reads only Path (for `gw affected`); it takes no
// opinion on how the service is built or run. Any other keys under the table
// (image, port, whatever your deploy tooling wants) are ignored by gw — they are
// yours to interpret.
type Service struct {
	// Path is the service's directory, relative to the workspace root (defaults
	// to the service name). Ownership for `gw affected` is by this directory.
	Path string `toml:"path" yaml:"path"`
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
