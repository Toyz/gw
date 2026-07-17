package gwext

import (
	"os"
	"strings"
)

// CIInfo is build metadata read from the CI runner's own environment —
// authoritative on the shallow, detached checkouts CI does, where `git describe`
// often can't resolve a tag. Every field is best-effort: outside CI (or when a
// variable is unset) it is left empty.
type CIInfo struct {
	Provider string // "github", "gitlab", or "" when not in CI
	Commit   string // full commit SHA
	Short    string // abbreviated SHA
	Ref      string // branch or tag short name
	Tag      string // tag name on a tag pipeline, else ""
	RunID    string // CI run / pipeline id
	Repo     string // "owner/repo" (GitHub) or "group/project" (GitLab)
	Actor    string // user that triggered the run
}

// CI detects the CI provider from the environment and returns its normalized
// build metadata. It reads the process environment (unlike Git, which shells
// out), so it is cheap and safe to call from any provider, command, or hook.
func CI() CIInfo {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return githubCI()
	case os.Getenv("GITLAB_CI") == "true":
		return gitlabCI()
	}
	return CIInfo{}
}

func githubCI() CIInfo {
	commit := os.Getenv("GITHUB_SHA")
	tag := ""
	if strings.HasPrefix(os.Getenv("GITHUB_REF"), "refs/tags/") {
		tag = os.Getenv("GITHUB_REF_NAME")
	}
	return CIInfo{
		Provider: "github",
		Commit:   commit,
		Short:    shortSHA(commit),
		Ref:      os.Getenv("GITHUB_REF_NAME"),
		Tag:      tag,
		RunID:    os.Getenv("GITHUB_RUN_ID"),
		Repo:     os.Getenv("GITHUB_REPOSITORY"),
		Actor:    os.Getenv("GITHUB_ACTOR"),
	}
}

func gitlabCI() CIInfo {
	commit := os.Getenv("CI_COMMIT_SHA")
	short := os.Getenv("CI_COMMIT_SHORT_SHA")
	if short == "" {
		short = shortSHA(commit)
	}
	return CIInfo{
		Provider: "gitlab",
		Commit:   commit,
		Short:    short,
		Ref:      os.Getenv("CI_COMMIT_REF_NAME"),
		Tag:      os.Getenv("CI_COMMIT_TAG"),
		RunID:    os.Getenv("CI_PIPELINE_ID"),
		Repo:     os.Getenv("CI_PROJECT_PATH"),
		Actor:    os.Getenv("GITLAB_USER_LOGIN"),
	}
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// CIStamp returns a build provider that stamps CI metadata into the given
// package via `-ldflags -X` — the CI-sourced analogue of GitStamp, reliable in
// pipelines where the git checkout is shallow or detached:
//
//	gwext.Provide(gwext.CIStamp("example.com/app/version"))
//
// Declare matching string vars in that package to receive the values:
//
//	package version
//	var (Provider, Commit, Short, Ref, Tag, RunID, Repo, Actor string)
//
// CI metadata is workspace-global, so pair CIStamp with Provide, not ProvideEach.
func CIStamp(pkg string) func(*Context) (BuildInfo, error) {
	return func(*Context) (BuildInfo, error) {
		ci := CI()
		p := strings.TrimSuffix(pkg, ".")
		return BuildInfo{Vars: map[string]string{
			p + ".Provider": ci.Provider,
			p + ".Commit":   ci.Commit,
			p + ".Short":    ci.Short,
			p + ".Ref":      ci.Ref,
			p + ".Tag":      ci.Tag,
			p + ".RunID":    ci.RunID,
			p + ".Repo":     ci.Repo,
			p + ".Actor":    ci.Actor,
		}}, nil
	}
}

// CIEnv returns a build provider that exports CI metadata as environment
// variables for every command gw runs: GW_CI_PROVIDER, GW_CI_COMMIT,
// GW_CI_SHORT, GW_CI_REF, GW_CI_TAG, GW_CI_RUN_ID, GW_CI_REPO, GW_CI_ACTOR.
//
//	gwext.Provide(gwext.CIEnv())
func CIEnv() func(*Context) (BuildInfo, error) {
	return func(*Context) (BuildInfo, error) {
		ci := CI()
		return BuildInfo{Env: map[string]string{
			"GW_CI_PROVIDER": ci.Provider,
			"GW_CI_COMMIT":   ci.Commit,
			"GW_CI_SHORT":    ci.Short,
			"GW_CI_REF":      ci.Ref,
			"GW_CI_TAG":      ci.Tag,
			"GW_CI_RUN_ID":   ci.RunID,
			"GW_CI_REPO":     ci.Repo,
			"GW_CI_ACTOR":    ci.Actor,
		}}, nil
	}
}
