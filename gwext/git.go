package gwext

import (
	"os/exec"
	"strings"
)

// GitInfo is git metadata for the workspace, resolved from the repo containing
// Root. Every field is best-effort: on any git error (not a repo, git missing,
// no commits) the corresponding field is left at its zero value.
type GitInfo struct {
	Commit string // full HEAD commit SHA
	Short  string // abbreviated HEAD SHA
	Branch string // current branch, or "HEAD" when detached
	Tag    string // `git describe --tags --always --dirty`, e.g. v1.2.3-4-gabc123
	Time   string // committer time of HEAD, RFC3339
	Dirty  bool   // the working tree has uncommitted changes
}

// Git computes GitInfo for the repo containing dir (typically c.Root). It never
// returns an error — unresolved values are simply empty — so it is safe to call
// from a provider without guarding for non-git checkouts.
func Git(dir string) GitInfo {
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return GitInfo{
		Commit: run("rev-parse", "HEAD"),
		Short:  run("rev-parse", "--short", "HEAD"),
		Branch: run("rev-parse", "--abbrev-ref", "HEAD"),
		Tag:    run("describe", "--tags", "--always", "--dirty"),
		Time:   run("show", "-s", "--format=%cI", "HEAD"),
		Dirty:  run("status", "--porcelain") != "",
	}
}

// GitStamp returns a ready-made build provider that stamps git metadata into the
// given package via `-ldflags -X`, so the user never writes the boilerplate:
//
//	gwext.Provide(gwext.GitStamp("example.com/app/version"))
//
// Declare matching string vars in that package to receive the values:
//
//	package version
//	var (Commit, Short, Branch, Tag, Time, Dirty string)
//
// Because `-X` only affects the module that actually links the package, the
// stamp lands on that module alone even though gw passes -ldflags to every
// build — no need to scope it yourself.
func GitStamp(pkg string) func(*Context) (BuildInfo, error) {
	return func(c *Context) (BuildInfo, error) {
		g := Git(c.Root)
		p := strings.TrimSuffix(pkg, ".")
		return BuildInfo{Vars: map[string]string{
			p + ".Commit": g.Commit,
			p + ".Short":  g.Short,
			p + ".Branch": g.Branch,
			p + ".Tag":    g.Tag,
			p + ".Time":   g.Time,
			p + ".Dirty":  boolString(g.Dirty),
		}}, nil
	}
}

// GitEnv returns a ready-made build provider that exports git metadata as
// environment variables for every command gw runs: GW_GIT_COMMIT, GW_GIT_SHORT,
// GW_GIT_BRANCH, GW_GIT_TAG, GW_GIT_TIME, GW_GIT_DIRTY.
//
//	gwext.Provide(gwext.GitEnv())
func GitEnv() func(*Context) (BuildInfo, error) {
	return func(c *Context) (BuildInfo, error) {
		g := Git(c.Root)
		return BuildInfo{Env: map[string]string{
			"GW_GIT_COMMIT": g.Commit,
			"GW_GIT_SHORT":  g.Short,
			"GW_GIT_BRANCH": g.Branch,
			"GW_GIT_TAG":    g.Tag,
			"GW_GIT_TIME":   g.Time,
			"GW_GIT_DIRTY":  boolString(g.Dirty),
		}}, nil
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
