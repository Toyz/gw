package gwext

import "testing"

func TestCIGitHub(t *testing.T) {
	t.Setenv("GITLAB_CI", "") // ensure GitHub wins detection
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_SHA", "abc1234567890def")
	t.Setenv("GITHUB_REF", "refs/tags/v1.2.3")
	t.Setenv("GITHUB_REF_NAME", "v1.2.3")
	t.Setenv("GITHUB_RUN_ID", "42")
	t.Setenv("GITHUB_REPOSITORY", "toyz/gw")
	t.Setenv("GITHUB_ACTOR", "toyz")

	ci := CI()
	if ci.Provider != "github" {
		t.Fatalf("provider = %q, want github", ci.Provider)
	}
	if ci.Short != "abc1234" {
		t.Errorf("short = %q, want abc1234", ci.Short)
	}
	if ci.Tag != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3 (tag ref)", ci.Tag)
	}
	if ci.Repo != "toyz/gw" || ci.RunID != "42" || ci.Actor != "toyz" {
		t.Errorf("repo/run/actor not parsed: %+v", ci)
	}
}

func TestCIGitHubBranchHasNoTag(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "main")

	if ci := CI(); ci.Tag != "" || ci.Ref != "main" {
		t.Fatalf("branch build should have empty Tag: %+v", ci)
	}
}

func TestCIGitLab(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_COMMIT_SHA", "def7654321")
	t.Setenv("CI_COMMIT_SHORT_SHA", "def7654")
	t.Setenv("CI_COMMIT_REF_NAME", "main")
	t.Setenv("CI_COMMIT_TAG", "")
	t.Setenv("CI_PIPELINE_ID", "99")
	t.Setenv("CI_PROJECT_PATH", "group/proj")

	ci := CI()
	if ci.Provider != "gitlab" || ci.Short != "def7654" || ci.Ref != "main" || ci.RunID != "99" || ci.Repo != "group/proj" {
		t.Fatalf("gitlab not parsed: %+v", ci)
	}
	if ci.Tag != "" {
		t.Errorf("branch build should have empty Tag, got %q", ci.Tag)
	}
}

func TestCINotDetected(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	if ci := CI(); ci.Provider != "" {
		t.Fatalf("want no provider outside CI, got %+v", ci)
	}
}
