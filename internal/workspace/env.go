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
// environment untouched (inherited) in the common zero-config case. For the
// finer-grained layers used when extensions contribute env, see ResolveConfigEnv
// and ResolveCLIEnv.
func ResolveEnv(root string, cfg Config, files, vars []string) ([]string, error) {
	cfgEnv, err := ResolveConfigEnv(root, cfg)
	if err != nil {
		return nil, err
	}
	cliEnv, err := ResolveCLIEnv(root, files, vars)
	if err != nil {
		return nil, err
	}
	if cfgEnv == nil && cliEnv == nil {
		return nil, nil
	}
	return append(cfgEnv, cliEnv...), nil
}

// ResolveConfigEnv returns the config-level overrides: inline cfg.Env (sorted)
// followed by cfg.EnvFiles in order. These are workspace-wide — gw applies them
// to every command, hook, and extension it spawns.
func ResolveConfigEnv(root string, cfg Config) ([]string, error) {
	var out []string
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
	files, err := readEnvFiles(root, cfg.EnvFiles)
	if err != nil {
		return nil, err
	}
	return append(out, files...), nil
}

// ResolveCLIEnv returns the per-invocation overrides from the --env-file (in
// order) and --env flags. These win over config and extension-provided env.
func ResolveCLIEnv(root string, files, vars []string) ([]string, error) {
	out, err := readEnvFiles(root, files)
	if err != nil {
		return nil, err
	}
	for _, v := range vars {
		if !strings.Contains(v, "=") {
			return nil, fmt.Errorf("--env %q must be KEY=VALUE", v)
		}
		out = append(out, v)
	}
	return out, nil
}

// readEnvFiles parses each dotenv file (relative to root) into KEY=VALUE entries.
func readEnvFiles(root string, files []string) ([]string, error) {
	var out []string
	for _, f := range files {
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
	return out, nil
}

// ParseEnvFile reads a dotenv-style file into KEY=VALUE entries, in file order.
//
// Edge cases handled: blank lines and full-line / inline `#` comments; a leading
// `export `; CRLF endings and a UTF-8 BOM; `KEY = value` spacing; `=` inside
// values; single quotes (fully literal), double quotes (with `\n \t \r \" \\ \$`
// escapes), and unquoted values. Variable expansion runs on unquoted and
// double-quoted values — `$VAR`, `${VAR}`, `${VAR:-default}`, `${VAR-default}`,
// with `\$` for a literal `$` — resolving against earlier entries in the same
// file, then the ambient process environment, else empty. Single-quoted values
// are never expanded. Unterminated quotes are an error.
func ParseEnvFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	seen := map[string]string{} // earlier-in-file values, for expansion
	lookup := func(name string) (string, bool) {
		if v, ok := seen[name]; ok {
			return v, true
		}
		return os.LookupEnv(name)
	}

	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		trimmed := strings.TrimLeft(strings.TrimRight(sc.Text(), "\r"), " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		eq := strings.IndexByte(trimmed, '=')
		if eq < 0 {
			return nil, fmt.Errorf("%s:%d: not KEY=VALUE: %q", path, n, trimmed)
		}
		key := strings.TrimRight(trimmed[:eq], " \t")
		if key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("%s:%d: invalid key %q", path, n, key)
		}
		val, err := parseEnvValue(trimmed[eq+1:], lookup)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		seen[key] = val
		out = append(out, key+"="+val)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseEnvValue resolves the substring after the first '=' into a final value,
// dispatching on the quoting style.
func parseEnvValue(raw string, lookup func(string) (string, bool)) (string, error) {
	v := strings.TrimLeft(raw, " \t")
	if v == "" {
		return "", nil
	}
	switch v[0] {
	case '\'':
		end := strings.IndexByte(v[1:], '\'')
		if end < 0 {
			return "", fmt.Errorf("unterminated single quote")
		}
		return v[1 : 1+end], nil // literal: no escapes, no expansion
	case '"':
		inner, err := scanDoubleQuoted(v[1:])
		if err != nil {
			return "", err
		}
		return expandEnv(inner, true, lookup), nil
	default:
		return expandEnv(strings.TrimRight(stripInlineComment(v), " \t"), false, lookup), nil
	}
}

// scanDoubleQuoted returns the contents of a double-quoted value (escape
// sequences left intact for expandEnv), given the text after the opening quote.
func scanDoubleQuoted(s string) (string, error) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // skip the escaped byte so \" does not close the quote
		case '"':
			return s[:i], nil
		}
	}
	return "", fmt.Errorf("unterminated double quote")
}

// stripInlineComment cuts an unquoted value at the first '#' that starts the
// value or follows whitespace, so URLs like http://x#frag keep their fragment.
func stripInlineComment(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '#' && (i == 0 || v[i-1] == ' ' || v[i-1] == '\t') {
			return v[:i]
		}
	}
	return v
}

// expandEnv processes escape sequences and $VAR / ${VAR} / ${VAR:-default}
// references. doubleQuoted selects the escape set; \$ always yields a literal $.
func expandEnv(s string, doubleQuoted bool, lookup func(string) (string, bool)) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			if esc, ok := escapeByte(s[i+1], doubleQuoted); ok {
				b.WriteByte(esc)
				i += 2
				continue
			}
			b.WriteByte(c)
			i++
			continue
		}
		if c == '$' {
			if name, def, op, adv := parseEnvRef(s[i:]); adv > 0 {
				val, ok := lookup(name)
				if (op == ":-" && (!ok || val == "")) || (op == "-" && !ok) {
					val = expandEnv(def, doubleQuoted, lookup)
				}
				b.WriteString(val)
				i += adv
				continue
			}
			b.WriteByte('$')
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// escapeByte maps the byte after a backslash to its literal, per quoting style.
func escapeByte(n byte, doubleQuoted bool) (byte, bool) {
	if doubleQuoted {
		switch n {
		case 'n':
			return '\n', true
		case 't':
			return '\t', true
		case 'r':
			return '\r', true
		case '"', '\\', '$':
			return n, true
		}
		return 0, false
	}
	switch n {
	case '\\', '$', '#', ' ':
		return n, true
	}
	return 0, false
}

// parseEnvRef parses a variable reference at the start of s (s[0]=='$'). It
// returns the name, default expression, operator ("", ":-", "-"), and bytes
// consumed; adv==0 means s does not begin a valid reference (treat $ literally).
func parseEnvRef(s string) (name, def, op string, adv int) {
	if len(s) < 2 {
		return "", "", "", 0
	}
	if s[1] == '{' {
		// Find the brace that matches s[1], counting nested "{"/"}" so a default
		// expression can itself contain a "${...}" reference.
		depth, end := 1, -1
		for i := 2; i < len(s); i++ {
			switch s[i] {
			case '{':
				depth++
			case '}':
				if depth--; depth == 0 {
					end = i
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			return "", "", "", 0
		}
		inner := s[2:end]
		// The operator is whatever immediately follows the variable name, so a
		// "-" default that itself contains ":-" isn't mistaken for a ":-" op.
		k := 0
		for k < len(inner) && isNameByte(inner[k]) {
			k++
		}
		name = inner[:k]
		switch rest := inner[k:]; {
		case rest == "":
			// bare ${NAME}
		case strings.HasPrefix(rest, ":-"):
			op, def = ":-", rest[2:]
		case rest[0] == '-':
			op, def = "-", rest[1:]
		default:
			return "", "", "", 0 // unsupported operator → treat $ literally
		}
		if !validRefName(name) {
			return "", "", "", 0
		}
		return name, def, op, end + 1
	}
	if !isNameStart(s[1]) {
		return "", "", "", 0
	}
	j := 1
	for j < len(s) && isNameByte(s[j]) {
		j++
	}
	return s[1:j], "", "", j
}

func validRefName(s string) bool {
	if s == "" || !isNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isNameByte(s[i]) {
			return false
		}
	}
	return true
}

func isNameStart(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isNameByte(b byte) bool { return isNameStart(b) || (b >= '0' && b <= '9') }
