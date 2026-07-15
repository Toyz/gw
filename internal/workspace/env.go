package workspace

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveEnv builds the ordered list of KEY=VALUE overrides to layer on top of
// the ambient process environment for commands gw runs. Precedence, lowest to
// highest: cfg.Env (inline) -> cfg.EnvFiles -> files (CLI --env-file) -> vars
// (CLI --env). The result is meant to be appended to os.Environ(); because a
// later duplicate key wins, this ordering yields the documented precedence.
//
// It returns nil when nothing is configured, so callers can leave a child's
// environment untouched (inherited) in the common zero-config case.
func ResolveEnv(root string, cfg Config, files, vars []string) ([]string, error) {
	var out []string

	// Inline config env, sorted for deterministic ordering.
	if len(cfg.Env) > 0 {
		keys := make([]string, 0, len(cfg.Env))
		for k := range cfg.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, k+"="+cfg.Env[k])
		}
	}

	// Config env files first, then CLI --env-file, each in listed order.
	for _, f := range append(append([]string{}, cfg.EnvFiles...), files...) {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		kv, err := ParseEnvFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, kv...)
	}

	// CLI --env KEY=VALUE wins over everything.
	for _, v := range vars {
		if !strings.Contains(v, "=") {
			return nil, fmt.Errorf("--env %q must be KEY=VALUE", v)
		}
		out = append(out, v)
	}

	return out, nil
}

// ParseEnvFile reads a dotenv-style file into KEY=VALUE entries. It skips blank
// lines and # comments, tolerates a leading "export ", and strips a single pair
// of surrounding quotes from the value (processing \n \t \r \" \\ escapes inside
// double quotes only). Values are taken literally otherwise — no variable
// interpolation and no inline-comment stripping (a value may contain #).
func ParseEnvFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil, fmt.Errorf("%s:%d: not KEY=VALUE: %q", path, n, line)
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, n)
		}
		out = append(out, key+"="+unquoteEnv(strings.TrimSpace(line[eq+1:])))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// unquoteEnv strips one pair of matching surrounding quotes from a dotenv value.
// Double quotes additionally process a small set of C-style escapes; single
// quotes are fully literal.
func unquoteEnv(s string) string {
	if len(s) < 2 {
		return s
	}
	switch {
	case s[0] == '\'' && s[len(s)-1] == '\'':
		return s[1 : len(s)-1]
	case s[0] == '"' && s[len(s)-1] == '"':
		return strings.NewReplacer(
			`\n`, "\n", `\t`, "\t", `\r`, "\r", `\"`, `"`, `\\`, `\`,
		).Replace(s[1 : len(s)-1])
	}
	return s
}
