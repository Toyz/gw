package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

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
}

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
