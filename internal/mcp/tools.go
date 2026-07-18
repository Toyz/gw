package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/toyz/gw/internal/workspace"
)

// Tool is one MCP tool: its schema and a handler that receives the raw JSON
// arguments and returns text output (or an error surfaced as an isError result).
type Tool struct {
	Name        string
	Description string
	InputSchema any
	Handler     func(args json.RawMessage) (string, error)
}

// rootArg is embedded by every tool's arguments: an optional workspace root.
type rootArg struct {
	Root string `json:"root"`
}

func schema(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var rootProp = map[string]any{
	"root": map[string]any{"type": "string", "description": "workspace root (default: nearest go.work, else cwd)"},
}

// resolveRoot resolves the workspace root from an optional argument.
func resolveRoot(arg string) (string, error) {
	if arg != "" {
		return filepath.Abs(arg)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if r, ok := workspace.FindRoot(cwd); ok {
		return r, nil
	}
	return cwd, nil
}

// loadWorkspace resolves the root and discovers modules.
func loadWorkspace(arg string) (root string, mods []workspace.Module, cfg workspace.Config, err error) {
	root, err = resolveRoot(arg)
	if err != nil {
		return "", nil, workspace.Config{}, err
	}
	cfg, err = workspace.LoadConfig(root)
	if err != nil {
		return "", nil, workspace.Config{}, err
	}
	mods, err = workspace.Discover(root, cfg)
	return root, mods, cfg, err
}

func toJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func allTools() []Tool {
	return []Tool{
		toolList(),
		toolDoctor(),
		toolLint(),
		toolGraph(),
		toolAffected(),
		toolSync(),
		toolTest(),
	}
}

func toolList() Tool {
	return Tool{
		Name:        "gw_list",
		Description: "List every module in the Go workspace with its module path, directory, go version, and direct external requires.",
		InputSchema: schema(rootProp),
		Handler: func(raw json.RawMessage) (string, error) {
			var a rootArg
			_ = json.Unmarshal(raw, &a)
			root, mods, _, err := loadWorkspace(a.Root)
			if err != nil {
				return "", err
			}
			type mod struct {
				Path      string            `json:"path"`
				Use       string            `json:"use"`
				GoVersion string            `json:"go,omitempty"`
				Requires  map[string]string `json:"requires,omitempty"`
			}
			out := make([]mod, 0, len(mods))
			for _, m := range mods {
				out = append(out, mod{m.Path, workspace.UsePath(root, m.Dir), m.GoVersion, m.Requires})
			}
			return toJSON(map[string]any{"root": root, "modules": out})
		},
	}
}

func toolDoctor() Tool {
	return Tool{
		Name:        "gw_doctor",
		Description: "Diagnose workspace health: missing/stale go.work, use entries with no go.mod, modules missing from go.work, un-hoisted replace directives, and dependency/directive version drift.",
		InputSchema: schema(rootProp),
		Handler: func(raw json.RawMessage) (string, error) {
			var a rootArg
			_ = json.Unmarshal(raw, &a)
			root, mods, cfg, err := loadWorkspace(a.Root)
			if err != nil {
				return "", err
			}
			issues := workspace.Diagnose(root, cfg, mods)
			type is struct {
				Severity string `json:"severity"`
				Message  string `json:"message"`
				Fix      string `json:"fix"`
			}
			var errs, warns, infos int
			list := make([]is, 0, len(issues))
			for _, i := range issues {
				switch i.Severity {
				case workspace.SevError:
					errs++
				case workspace.SevWarn:
					warns++
				case workspace.SevInfo:
					infos++
				}
				list = append(list, is{string(i.Severity), i.Msg, i.Fix})
			}
			return toJSON(map[string]any{
				"healthy": len(issues) == 0,
				"errors":  errs, "warnings": warns, "infos": infos,
				"issues": list,
			})
		},
	}
}

func toolLint() Tool {
	return Tool{
		Name:        "gw_lint",
		Description: "Report dependencies required at more than one version across modules, plus mismatched go/toolchain directives. Empty means the workspace is consistent.",
		InputSchema: schema(rootProp),
		Handler: func(raw json.RawMessage) (string, error) {
			var a rootArg
			_ = json.Unmarshal(raw, &a)
			_, mods, _, err := loadWorkspace(a.Root)
			if err != nil {
				return "", err
			}
			ms := workspace.Lint(mods)
			type mismatch struct {
				Dep      string              `json:"dep"`
				Versions map[string][]string `json:"versions"`
			}
			out := make([]mismatch, 0, len(ms))
			for _, m := range ms {
				out = append(out, mismatch{m.Dep, m.Versions})
			}
			return toJSON(map[string]any{"consistent": len(ms) == 0, "mismatches": out})
		},
	}
}

func toolGraph() Tool {
	return Tool{
		Name:        "gw_graph",
		Description: "Return the intra-workspace module dependency graph: nodes and directed edges (A depends on B).",
		InputSchema: schema(rootProp),
		Handler: func(raw json.RawMessage) (string, error) {
			var a rootArg
			_ = json.Unmarshal(raw, &a)
			_, mods, _, err := loadWorkspace(a.Root)
			if err != nil {
				return "", err
			}
			g := workspace.BuildGraph(mods)
			nodes := make([]string, 0, len(g.Modules))
			for _, m := range g.Modules {
				nodes = append(nodes, m.Path)
			}
			type edge struct {
				From string `json:"from"`
				To   string `json:"to"`
			}
			edges := make([]edge, 0)
			for _, e := range g.Edges() {
				edges = append(edges, edge{e[0], e[1]})
			}
			return toJSON(map[string]any{"nodes": nodes, "edges": edges})
		},
	}
}

func toolAffected() Tool {
	return Tool{
		Name:        "gw_affected",
		Description: "Given a git ref, map changed files to their owning modules and walk the dependency graph to every impacted module. Use for selective CI/test runs. Returns seeds (directly changed) and impacted (transitive).",
		InputSchema: schema(map[string]any{
			"root":  rootProp["root"],
			"since": map[string]any{"type": "string", "description": "git ref to diff against, e.g. main or HEAD~1"},
		}, "since"),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				Root  string `json:"root"`
				Since string `json:"since"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", err
			}
			if a.Since == "" {
				return "", fmt.Errorf("`since` is required (a git ref)")
			}
			root, mods, cfg, err := loadWorkspace(a.Root)
			if err != nil {
				return "", err
			}
			gitRoot, err := workspace.GitRoot(root)
			if err != nil {
				return "", fmt.Errorf("not a git repository: %w", err)
			}
			changed, err := workspace.ChangedFiles(gitRoot, a.Since)
			if err != nil {
				return "", fmt.Errorf("git diff against %q: %w", a.Since, err)
			}
			g := workspace.BuildGraph(mods)
			seeds, impacted := workspace.AffectedModules(g, mods, changed)
			projects := workspace.AffectedProjects(root, cfg.Projects, changed)
			sort.Strings(seeds)
			sort.Strings(impacted)
			return toJSON(map[string]any{"since": a.Since, "seeds": seeds, "impacted": impacted, "projects": projects})
		},
	}
}

func toolSync() Tool {
	return Tool{
		Name:        "gw_sync",
		Description: "Regenerate go.work's use set from discovered modules. With check=true, only report whether it is out of date (no write).",
		InputSchema: schema(map[string]any{
			"root":  rootProp["root"],
			"check": map[string]any{"type": "boolean", "description": "report drift without writing (default false)"},
		}),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				Root  string `json:"root"`
				Check bool   `json:"check"`
			}
			_ = json.Unmarshal(raw, &a)
			gwArgs := []string{"sync", "--no-work-sync"}
			if a.Check {
				gwArgs = []string{"sync", "--check", "--no-work-sync"}
			}
			return runGW(a.Root, gwArgs...)
		},
	}
}

func toolTest() Tool {
	return Tool{
		Name:        "gw_test",
		Description: "Run `go test` across every module in the workspace and return the combined output and pass/fail summary.",
		InputSchema: schema(map[string]any{
			"root":     rootProp["root"],
			"packages": map[string]any{"type": "string", "description": "package pattern (default ./...)"},
		}),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				Root     string `json:"root"`
				Packages string `json:"packages"`
			}
			_ = json.Unmarshal(raw, &a)
			args := []string{"test"}
			if a.Packages != "" {
				args = append(args, a.Packages)
			}
			return runGW(a.Root, args...)
		},
	}
}

// runGW invokes the running gw binary for action tools, so they reuse the full
// command behavior (providers, hooks, exec). Output is captured and returned.
func runGW(root string, args ...string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	r, err := resolveRoot(root)
	if err != nil {
		return "", err
	}
	full := append([]string{"-C", r}, args...)
	cmd := exec.CommandContext(context.Background(), self, full...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()
	out := buf.String()
	if runErr != nil {
		return out, fmt.Errorf("gw %v exited: %v\n%s", args, runErr, out)
	}
	if out == "" {
		out = "(no output)"
	}
	return out, nil
}
