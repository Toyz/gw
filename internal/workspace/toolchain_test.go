package workspace

import (
	"strings"
	"testing"
)

func TestTaskCommand(t *testing.T) {
	cfg := Config{Toolchains: map[string]map[string]string{
		"uv":   {"test": "uv run pytest"},
		"rust": {"test": "cargo nextest run"}, // overrides the first-party rust test
	}}

	goUnit := Unit{Name: "example.com/api", Dir: "/ws/api", Toolchain: "go"}
	rustUnit := Unit{Name: "sat", Dir: "/ws/sat", Toolchain: "rust"}
	uvUnit := Unit{Name: "ingest", Dir: "/ws/py", Toolchain: "uv"}
	taskUnit := Unit{Name: "sat", Dir: "/ws/sat", Toolchain: "rust", Tasks: map[string]string{"test": "just test"}}
	blubUnit := Unit{Name: "x", Dir: "/ws/x", Toolchain: "blub"}

	// first-party go -> argv (keeps injection path)
	if argv, shell, err := TaskCommand(cfg, goUnit, "build"); err != nil || shell != "" || strings.Join(argv, " ") != "go build ./..." {
		t.Errorf("go build = (%v, %q, %v)", argv, shell, err)
	}
	// first-party rust vet -> cargo clippy argv (no override for vet)
	if argv, _, err := TaskCommand(cfg, rustUnit, "vet"); err != nil || strings.Join(argv, " ") != "cargo clippy" {
		t.Errorf("rust vet = %v (%v)", argv, err)
	}
	// [toolchains.rust] test overrides the built-in -> shell
	if _, shell, err := TaskCommand(cfg, rustUnit, "test"); err != nil || shell != "cargo nextest run" {
		t.Errorf("rust test override = %q (%v)", shell, err)
	}
	// user toolchain uv -> shell from config
	if _, shell, err := TaskCommand(cfg, uvUnit, "test"); err != nil || shell != "uv run pytest" {
		t.Errorf("uv test = %q (%v)", shell, err)
	}
	// per-project [tasks] beats everything
	if _, shell, err := TaskCommand(cfg, taskUnit, "test"); err != nil || shell != "just test" {
		t.Errorf("project task override = %q (%v)", shell, err)
	}
	// undefined toolchain -> clear error
	if _, _, err := TaskCommand(cfg, blubUnit, "test"); err == nil || !strings.Contains(err.Error(), "not defined") {
		t.Errorf("undefined toolchain error = %v", err)
	}
	// known toolchain missing a verb (rust generate) -> error
	if _, _, err := TaskCommand(cfg, rustUnit, "generate"); err == nil || !strings.Contains(err.Error(), "no \"generate\"") {
		t.Errorf("rust generate error = %v", err)
	}
}

func TestUnits(t *testing.T) {
	mods := []Module{{Path: "example.com/api", Dir: "/ws/svc/api"}}
	projects := map[string]Project{
		"sat":     {Path: "sat", Toolchain: "rust"},
		"overlap": {Path: "svc/api"}, // same dir as the module -> module wins
	}
	units, overlaps := Units("/ws", mods, projects)
	byName := map[string]Unit{}
	for _, u := range units {
		byName[u.Name] = u
	}
	if u, ok := byName["example.com/api"]; !ok || u.Toolchain != "go" || !u.IsModule {
		t.Errorf("go module unit = %+v", u)
	}
	if u, ok := byName["sat"]; !ok || u.Toolchain != "rust" || u.Dir != "/ws/sat" {
		t.Errorf("rust project unit = %+v", u)
	}
	if _, ok := byName["overlap"]; ok {
		t.Error("overlapping project should be dropped (module wins)")
	}
	if len(overlaps) != 1 || overlaps[0] != "overlap" {
		t.Errorf("overlaps = %v", overlaps)
	}
}
