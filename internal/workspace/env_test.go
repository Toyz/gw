package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lastVal returns the winning (last) value for key in an override slice, mirroring
// how os/exec resolves duplicate keys.
func lastVal(env []string, key string) (string, bool) {
	val, ok := "", false
	for _, e := range env {
		if strings.HasPrefix(e, key+"=") {
			val, ok = e[len(key)+1:], true
		}
	}
	return val, ok
}

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# a comment\n" +
		"\n" +
		"export FOO=bar\n" +
		"BAR = \"with spaces\"\n" +
		"BAZ='single'\n" +
		"QUX=\"line\\nbreak\"\n" +
		"EMPTY=\n" +
		"HASH=a#b\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}
	want := map[string]string{
		"FOO":   "bar",
		"BAR":   "with spaces",
		"BAZ":   "single",
		"QUX":   "line\nbreak", // escape processed inside double quotes
		"EMPTY": "",
		"HASH":  "a#b", // no inline-comment stripping
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %q", len(got), len(want), got)
	}
	for k, w := range want {
		if v, ok := lastVal(got, k); !ok || v != w {
			t.Errorf("%s = %q (ok=%v), want %q", k, v, ok, w)
		}
	}
}

func TestParseEnvFileExpansion(t *testing.T) {
	t.Setenv("HOST", "example.com")
	t.Setenv("EMPTY_IN_ENV", "")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	// Note: \r checks CRLF; the leading BOM is prepended below.
	content := "BASE=/opt\r\n" +
		"BIN=${BASE}/bin\n" + // ${VAR} refs an earlier line
		"URL=https://$HOST/api\n" + // $VAR refs the process env
		"FRAG=http://x#keep\n" + // '#' without leading space stays
		"NOTE=value # trailing comment\n" + // inline comment stripped
		"LITERAL='$HOST and #hash'\n" + // single quotes: no expand, no comment
		"ESCAPED=\"cost is \\$5\"\n" + // \$ -> literal $
		"DEFAULT=${MISSING:-fallback}\n" + // default when unset
		"EMPTY=${EMPTY_IN_ENV:-used}\n" + // :- also triggers on empty
		"KEEPEMPTY=${EMPTY_IN_ENV-unused}\n" // - keeps the empty value
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, content...), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}
	want := map[string]string{
		"BASE":      "/opt",
		"BIN":       "/opt/bin",
		"URL":       "https://example.com/api",
		"FRAG":      "http://x#keep",
		"NOTE":      "value",
		"LITERAL":   "$HOST and #hash",
		"ESCAPED":   "cost is $5",
		"DEFAULT":   "fallback",
		"EMPTY":     "used",
		"KEEPEMPTY": "",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %q", len(got), len(want), got)
	}
	for k, w := range want {
		if v, ok := lastVal(got, k); !ok || v != w {
			t.Errorf("%s = %q (ok=%v), want %q", k, v, ok, w)
		}
	}
}

func TestParseEnvFileUnterminatedQuote(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"K=\"no close\n", "K='no close\n"} {
		path := filepath.Join(dir, "bad.env")
		if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ParseEnvFile(path); err == nil {
			t.Errorf("expected error for unterminated quote in %q", bad)
		}
	}
}

func TestParseEnvFileBadLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("VALID=1\nNOEQUALS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvFile(path); err == nil {
		t.Fatal("expected error for a line without '='")
	}
}

func TestResolveEnvPrecedence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("B=file\nSHARED=file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli.env"), []byte("C=clifile\nSHARED=clifile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Env:      map[string]string{"A": "cfg", "SHARED": "cfg"},
		EnvFiles: []string{".env"},
	}
	env, err := ResolveEnv(root, cfg, []string{"cli.env"}, []string{"D=clivar", "SHARED=clivar"})
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}

	for k, w := range map[string]string{"A": "cfg", "B": "file", "C": "clifile", "D": "clivar"} {
		if v, ok := lastVal(env, k); !ok || v != w {
			t.Errorf("%s = %q (ok=%v), want %q", k, v, ok, w)
		}
	}
	// Highest precedence layer (CLI --env) must win the shared key.
	if v, _ := lastVal(env, "SHARED"); v != "clivar" {
		t.Errorf("SHARED = %q, want clivar (CLI --env wins)", v)
	}
}

func TestResolveEnvEmpty(t *testing.T) {
	env, err := ResolveEnv(t.TempDir(), Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if env != nil {
		t.Errorf("zero-config ResolveEnv = %q, want nil", env)
	}
}

func TestResolveEnvBadVar(t *testing.T) {
	if _, err := ResolveEnv(t.TempDir(), Config{}, nil, []string{"NOEQUALS"}); err == nil {
		t.Fatal("expected error for --env without '='")
	}
}

func TestResolveEnvMissingFile(t *testing.T) {
	if _, err := ResolveEnv(t.TempDir(), Config{}, []string{"nope.env"}, nil); err == nil {
		t.Fatal("expected error for a missing env file")
	}
}
