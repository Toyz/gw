package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigTOML(t *testing.T) {
	root := t.TempDir()
	toml := "" +
		"root = \".\"\n" +
		"ignore = [\"examples/**\"]\n" +
		"env_files = [\".env\"]\n" +
		"\n" +
		"[pins]\n" +
		"\"github.com/foo/bar\" = \"v1.4.0\"\n" +
		"\n" +
		"[env]\n" +
		"FOO = \"bar\"\n"
	if err := os.WriteFile(filepath.Join(root, "gw.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Root != "." || len(cfg.Ignore) != 1 || cfg.Ignore[0] != "examples/**" {
		t.Errorf("root/ignore not parsed: %+v", cfg)
	}
	if cfg.Pins["github.com/foo/bar"] != "v1.4.0" {
		t.Errorf("pins not parsed: %+v", cfg.Pins)
	}
	if cfg.Env["FOO"] != "bar" {
		t.Errorf("env not parsed: %+v", cfg.Env)
	}
	if len(cfg.EnvFiles) != 1 || cfg.EnvFiles[0] != ".env" {
		t.Errorf("env_files not parsed: %+v", cfg.EnvFiles)
	}
}

func TestLoadConfigCommandsHooks(t *testing.T) {
	root := t.TempDir()
	toml := "" +
		"[commands.boot]\n" +
		"desc = \"build in order\"\n" +
		"steps = [\"api:build\", \"web:build\"]\n" +
		"\n" +
		"[commands.web]\n" +
		"steps = [\"npm run dev\"]\n" +
		"dir = \"web\"\n" +
		"\n" +
		"[hooks.pre-build]\n" +
		"steps = [\"sqlc generate\"]\n"
	if err := os.WriteFile(filepath.Join(root, "gw.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	boot, ok := cfg.Commands["boot"]
	if !ok || boot.Desc != "build in order" || len(boot.Steps) != 2 || boot.Steps[0] != "api:build" {
		t.Errorf("commands.boot not parsed: %+v (ok=%v)", boot, ok)
	}
	if web := cfg.Commands["web"]; len(web.Steps) != 1 || web.Steps[0] != "npm run dev" || web.Dir != "web" {
		t.Errorf("commands.web not parsed: %+v", web)
	}
	if h, ok := cfg.Hooks["pre-build"]; !ok || len(h.Steps) != 1 || h.Steps[0] != "sqlc generate" {
		t.Errorf("hooks.pre-build not parsed: %+v (ok=%v)", h, ok)
	}
	if boot.Empty() {
		t.Error("boot should not be Empty")
	}
	if !(ConfigCommand{}).Empty() {
		t.Error("zero ConfigCommand should be Empty")
	}
}

func TestLoadConfigYAML(t *testing.T) {
	root := t.TempDir()
	yaml := "" +
		"root: .\n" +
		"ignore:\n" +
		"  - \"examples/**\"\n" +
		"pins:\n" +
		"  github.com/foo/bar: v1.4.0\n" +
		"env:\n" +
		"  FOO: bar\n" +
		"envFiles:\n" +
		"  - .env\n"
	if err := os.WriteFile(filepath.Join(root, "gw.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Pins["github.com/foo/bar"] != "v1.4.0" || cfg.Env["FOO"] != "bar" || len(cfg.EnvFiles) != 1 {
		t.Errorf("YAML not parsed: %+v", cfg)
	}
}

func TestLoadConfigTOMLPreferred(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gw.toml"), []byte("root = \"from-toml\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gw.yaml"), []byte("root: from-yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Root != "from-toml" {
		t.Errorf("root = %q, want from-toml (TOML must win over YAML)", cfg.Root)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if cfg.Root != "" || cfg.Ignore != nil || cfg.Env != nil {
		t.Errorf("missing config should be zero value: %+v", cfg)
	}
}
