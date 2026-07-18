package workspace

import "fmt"

// First-party toolchains: intentional built-in resolvers (argv), overridable via
// [toolchains.go]/[toolchains.rust]. Everything else is user [toolchains] data.
var goVerbArgv = map[string][]string{
	"build":    {"go", "build", "./..."},
	"test":     {"go", "test", "./..."},
	"vet":      {"go", "vet", "./..."},
	"generate": {"go", "generate", "./..."},
	"tidy":     {"go", "mod", "tidy"},
	"run":      {"go", "run", "."},
}

var rustVerbArgv = map[string][]string{
	"build": {"cargo", "build"},
	"test":  {"cargo", "test"},
	"vet":   {"cargo", "clippy"},
	"run":   {"cargo", "run"},
	// generate/tidy have no cargo analog — resolve to an error unless overridden.
}

func goTask(verb string) ([]string, bool)   { a, ok := goVerbArgv[verb]; return a, ok }
func rustTask(verb string) ([]string, bool) { a, ok := rustVerbArgv[verb]; return a, ok }

// TaskCommand resolves how a unit runs a verb. It returns EITHER argv (a
// first-party go/rust command, run directly so build-provider injection can apply
// to go) OR shell (a shell command, run via `sh -c` in the unit dir). Resolution,
// most specific first: per-project [tasks] override, then [toolchains.<tc>]
// (which can override the built-in go/rust verbs), then the first-party default,
// else a clear error.
func TaskCommand(cfg Config, unit Unit, verb string) (argv []string, shell string, err error) {
	// 1. per-project [tasks] override
	if s := unit.Tasks[verb]; s != "" {
		return nil, s, nil
	}
	tc := unit.Toolchain
	if tc == "" {
		tc = "go"
	}
	// 2. [toolchains.<tc>][verb] — user language, or override of a built-in verb
	if s := cfg.Toolchains[tc][verb]; s != "" {
		return nil, s, nil
	}
	// 3. first-party defaults
	switch tc {
	case "go":
		if a, ok := goTask(verb); ok {
			return a, "", nil
		}
	case "rust":
		if a, ok := rustTask(verb); ok {
			return a, "", nil
		}
	}
	// 4. error — undefined toolchain vs known toolchain missing this verb
	if tc != "go" && tc != "rust" {
		if _, defined := cfg.Toolchains[tc]; !defined {
			return nil, "", fmt.Errorf("toolchain %q is not defined; add [toolchains.%s] to gw.toml", tc, tc)
		}
	}
	return nil, "", fmt.Errorf("toolchain %q defines no %q; add [toolchains.%s] %s = \"...\" or a [projects.<name>.tasks] %s override", tc, verb, tc, verb, verb)
}

// KnownVerbs is the fixed verb set gw dispatches (used to validate step verbs).
var KnownVerbs = map[string]bool{
	"build": true, "test": true, "vet": true, "generate": true, "tidy": true, "run": true,
}
