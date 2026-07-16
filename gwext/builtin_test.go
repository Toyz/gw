package gwext

import (
	"os"
	"strings"
	"testing"
)

// Builtin needs gw to advertise its path; without GW_BIN it must error clearly
// rather than exec something arbitrary.
func TestBuiltinRequiresGWBin(t *testing.T) {
	old, had := os.LookupEnv("GW_BIN")
	os.Unsetenv("GW_BIN")
	t.Cleanup(func() {
		if had {
			os.Setenv("GW_BIN", old)
		}
	})

	c := &Context{Root: t.TempDir()}
	err := c.Builtin("run", "--", "echo", "hi")
	if err == nil {
		t.Fatal("expected error when GW_BIN is unset")
	}
	if !strings.Contains(err.Error(), "GW_BIN") {
		t.Fatalf("error should mention GW_BIN, got %v", err)
	}
}
