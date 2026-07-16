package gwext

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

// Flag declares a typed argument for a Command. Build them with Str/Bool/Int and
// pass them after the handler; gw parses c.Args into them, the handler reads
// typed values via c.String/c.Bool/c.Int, and they show up in `gw ext list` and
// `gw <cmd> --help`.
type Flag struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"` // "string" | "bool" | "int"
	Def     string   `json:"default,omitempty"`
	Help    string   `json:"help,omitempty"`
	Aliases []string `json:"aliases,omitempty"` // alternate names, e.g. "n" for "name"
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

// Strs declares a repeatable string-slice flag. Pass it multiple times
// (--tag a --tag b) and/or comma-separated (--tag a,b); both accumulate. Read
// the collected values with c.Strings(name).
func Strs(name, help string) Flag {
	return Flag{Name: name, Kind: "strings", Help: help}
}

// stringsValue is a flag.Value that accumulates repeated and comma-separated
// occurrences into one slice.
type stringsValue struct{ p *[]string }

func (s stringsValue) String() string {
	if s.p == nil {
		return ""
	}
	return strings.Join(*s.p, ",")
}

func (s stringsValue) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*s.p = append(*s.p, part)
		}
	}
	return nil
}

// Alias adds one or more alternate names for a flag. Any alias sets the same
// value; the handler still reads it by the canonical Name (c.String/Bool/Int).
// e.g. gwext.Str("name", "world", "who").Alias("n").
func (f Flag) Alias(names ...string) Flag {
	f.Aliases = append(f.Aliases, names...)
	return f
}

// parseFlags binds decls onto a FlagSet, parses argv, and returns the typed
// value map plus the leftover positional args. On -h/--help it prints usage and
// exits 0; on a bad flag it prints the error+usage (via the FlagSet) and exits 1.
func parseFlags(name string, decls []Flag, argv []string) (map[string]any, []string) {
	fs := flag.NewFlagSet("gw "+name, flag.ContinueOnError)
	vals := map[string]any{}
	// Register the canonical name and every alias against the same value pointer,
	// so passing either sets it; the handler reads by canonical Name.
	for _, d := range decls {
		names := append([]string{d.Name}, d.Aliases...)
		switch d.Kind {
		case "bool":
			p := new(bool)
			*p = d.Def == "true"
			for _, n := range names {
				fs.BoolVar(p, n, *p, d.Help)
			}
			vals[d.Name] = p
		case "int":
			p := new(int)
			*p, _ = strconv.Atoi(d.Def)
			for _, n := range names {
				fs.IntVar(p, n, *p, d.Help)
			}
			vals[d.Name] = p
		case "strings":
			p := new([]string)
			for _, n := range names {
				fs.Var(stringsValue{p}, n, d.Help)
			}
			vals[d.Name] = p
		default:
			p := new(string)
			*p = d.Def
			for _, n := range names {
				fs.StringVar(p, n, *p, d.Help)
			}
			vals[d.Name] = p
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

// Strings returns a declared string-slice flag's accumulated values (nil if
// undeclared or never passed).
func (c *Context) Strings(name string) []string {
	if v, ok := c.flags[name].(*[]string); ok {
		return *v
	}
	return nil
}
