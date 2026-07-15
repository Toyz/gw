package gwext

import (
	"flag"
	"os"
	"strconv"
)

// Flag declares a typed argument for a Command. Build them with Str/Bool/Int and
// pass them after the handler; gw parses c.Args into them, the handler reads
// typed values via c.String/c.Bool/c.Int, and they show up in `gw ext list` and
// `gw <cmd> --help`.
type Flag struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "string" | "bool" | "int"
	Def  string `json:"default,omitempty"`
	Help string `json:"help,omitempty"`
}

// Str declares a string flag with a default value.
func Str(name, def, help string) Flag {
	return Flag{Name: name, Kind: "string", Def: def, Help: help}
}

// Bool declares a boolean flag (default false).
func Bool(name, help string) Flag {
	return Flag{Name: name, Kind: "bool", Def: "false", Help: help}
}

// Int declares an integer flag with a default value.
func Int(name string, def int, help string) Flag {
	return Flag{Name: name, Kind: "int", Def: strconv.Itoa(def), Help: help}
}

// parseFlags binds decls onto a FlagSet, parses argv, and returns the typed
// value map plus the leftover positional args. On -h/--help it prints usage and
// exits 0; on a bad flag it prints the error+usage (via the FlagSet) and exits 1.
func parseFlags(name string, decls []Flag, argv []string) (map[string]any, []string) {
	fs := flag.NewFlagSet("gw "+name, flag.ContinueOnError)
	vals := map[string]any{}
	for _, d := range decls {
		switch d.Kind {
		case "bool":
			vals[d.Name] = fs.Bool(d.Name, d.Def == "true", d.Help)
		case "int":
			n, _ := strconv.Atoi(d.Def)
			vals[d.Name] = fs.Int(d.Name, n, d.Help)
		default:
			vals[d.Name] = fs.String(d.Name, d.Def, d.Help)
		}
	}
	if err := fs.Parse(argv); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
	return vals, fs.Args()
}

// String returns a declared string flag's value ("" if undeclared).
func (c *Context) String(name string) string {
	if v, ok := c.flags[name].(*string); ok {
		return *v
	}
	return ""
}

// Bool returns a declared bool flag's value (false if undeclared).
func (c *Context) Bool(name string) bool {
	if v, ok := c.flags[name].(*bool); ok {
		return *v
	}
	return false
}

// Int returns a declared int flag's value (0 if undeclared).
func (c *Context) Int(name string) int {
	if v, ok := c.flags[name].(*int); ok {
		return *v
	}
	return 0
}
