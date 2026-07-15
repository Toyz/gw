package workspace

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the optional per-workspace config filename, read from the root.
const ConfigFile = "gw.yaml"

// Config is the optional gw.yaml. Every field is optional; a missing file
// yields the zero value plus the built-in defaults applied by LoadConfig.
type Config struct {
	// Root overrides the workspace root (relative to the file, or absolute).
	Root string `yaml:"root"`
	// Ignore is a list of path globs (matched against the path relative to the
	// root, using filepath.Match semantics per segment) to skip during discovery.
	Ignore []string `yaml:"ignore"`
	// Pins forces specific dependency versions when running `lint --fix`.
	// Keyed by module path, e.g. "github.com/foo/bar": "v1.4.0".
	Pins map[string]string `yaml:"pins"`
}

// defaultIgnores are directory names never walked into during discovery.
var defaultIgnores = []string{".git", "vendor", "testdata", "node_modules", ".idea", ".vscode"}

// LoadConfig reads gw.yaml from root if present. A missing file is not an error:
// it returns a zero Config. Parse/read errors are returned.
func LoadConfig(root string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
