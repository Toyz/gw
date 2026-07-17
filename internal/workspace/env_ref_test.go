package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// envValue returns the value of key from a KEY=VALUE slice.
func envValue(kvs []string, key string) (string, bool) {
	for _, kv := range kvs {
		if len(kv) > len(key) && kv[:len(key)] == key && kv[len(key)] == '=' {
			return kv[len(key)+1:], true
		}
	}
	return "", false
}

func TestParseEnvRefDefaults(t *testing.T) {
	// Ensure the referenced vars are unset so defaults are taken.
	for _, v := range []string{"UNSET_VAR", "UNSET_A", "UNSET_B"} {
		os.Unsetenv(v)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "" +
		"A=${UNSET_VAR-a:-b}\n" + // "-" default whose value contains ":-"
		"B=${UNSET_A:-${UNSET_B:-fallback}}\n" + // nested ${...} default
		"C=${UNSET_A:-plain}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	kvs, err := ParseEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "a:-b", "B": "fallback", "C": "plain"}
	for k, exp := range want {
		if got, _ := envValue(kvs, k); got != exp {
			t.Errorf("%s=%q, want %q", k, got, exp)
		}
	}
}
