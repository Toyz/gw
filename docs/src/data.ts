export const REPO = "https://github.com/toyz/gw";
export const SLUG = "toyz/gw";
export const INSTALL = "go install github.com/toyz/gw@latest";

export interface Feature {
  icon: string;
  title: string;
  body: string;
}

export const FEATURES: Feature[] = [
  {
    icon: "git-merge",
    title: "Maintains go.work",
    body: "Discovers every module and keeps the use set in sync. gw init hoists stray replace directives up out of each go.mod.",
  },
  {
    icon: "shield-check",
    title: "Lints versions",
    body: "Flags dependencies pinned at different versions across modules, and go/toolchain drift. --fix aligns them.",
  },
  {
    icon: "terminal",
    title: "Runs everywhere",
    body: "build / test / vet / tidy / run across every module, serial or -p parallel, with a per-module summary and real exit code.",
  },
  {
    icon: "git-branch",
    title: "Knows what changed",
    body: "gw affected --since main maps a diff to owning modules, then walks the DAG to everything impacted. Feed it to CI.",
  },
  {
    icon: "package",
    title: "Extends in Go",
    body: "A compiled .gw/build.go adds commands, hooks, and build providers — and can override or hide builtins.",
  },
  {
    icon: "layers",
    title: "Barely configured",
    body: "Works with none. A gw.toml (or gw.yaml) adds ignore globs, version pins, and opt-in environment injection.",
  },
];

export interface Command {
  name: string;
  desc: string;
}

export interface CommandGroup {
  label: string;
  items: Command[];
}

export const COMMAND_GROUPS: CommandGroup[] = [
  {
    label: "Workspace",
    items: [
      { name: "gw init", desc: "Create go.work and hoist replace directives out of each go.mod." },
      { name: "gw sync", desc: "Regenerate the use set from discovered modules. --check fails stale CI." },
      { name: "gw lint", desc: "Report cross-module version drift; --fix aligns it." },
      { name: "gw doctor", desc: "One-pass health check: stale go.work, un-hoisted replaces, drift." },
    ],
  },
  {
    label: "Across every module",
    items: [
      { name: "gw build", desc: "go build in every module (default ./...)." },
      { name: "gw test", desc: "go test everywhere; -p parallel, go flags pass through." },
      { name: "gw vet", desc: "go vet across the workspace." },
      { name: "gw generate", desc: "go generate across the workspace." },
      { name: "gw tidy", desc: "go mod tidy in every module." },
      { name: "gw run -- <cmd>", desc: "Run any command in each module's directory." },
    ],
  },
  {
    label: "Inspect",
    items: [
      { name: "gw list", desc: "List modules; -v for versions, --json for tooling." },
      { name: "gw graph", desc: "The dependency DAG — text, --dot (Graphviz), or --json." },
      { name: "gw affected --since <ref>", desc: "Diff → owning modules → everything impacted." },
    ],
  },
  {
    label: "Modules & extensions",
    items: [
      { name: "gw add / remove <path>", desc: "Add or drop a single module's use directive." },
      { name: "gw ext", desc: "Scaffold, build, and list the .gw/build.go extension." },
    ],
  },
];

export interface Line {
  kind: "prompt" | "add" | "ok" | "path" | "dim";
  text: string;
}

// A realistic gw session for the hero terminal.
export const SESSION: Line[] = [
  { kind: "prompt", text: "gw sync" },
  { kind: "add", text: "+ ./services/gateway" },
  { kind: "dim", text: "wrote go.work: 7 module(s)" },
  { kind: "prompt", text: "gw affected --since main" },
  { kind: "path", text: "example.com/api" },
  { kind: "path", text: "example.com/gateway" },
  { kind: "prompt", text: "gw test -p" },
  { kind: "ok", text: "ok    example.com/api        142ms" },
  { kind: "ok", text: "ok    example.com/gateway    98ms" },
  { kind: "dim", text: "2 module(s), 0 failed" },
];
