package gwext

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo makes a temp git repo with one commit and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"checkout", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed (%v): %s", args, err, out)
		}
	}
	return dir
}

func TestGit(t *testing.T) {
	dir := initRepo(t)
	g := Git(dir)
	if len(g.Commit) != 40 {
		t.Errorf("Commit = %q, want a 40-char SHA", g.Commit)
	}
	if g.Short == "" || len(g.Short) >= len(g.Commit) {
		t.Errorf("Short = %q, want an abbreviated SHA", g.Short)
	}
	if g.Branch != "main" {
		t.Errorf("Branch = %q, want main", g.Branch)
	}
	if g.Dirty {
		t.Error("Dirty = true, want false for a clean tree")
	}

	// Introduce an untracked file -> dirty.
	if err := exec.Command("git", "-C", dir, "config", "core.excludesFile", "/dev/null").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Git(dir).Dirty {
		t.Error("Dirty = false after adding an untracked file, want true")
	}
}

func TestGitNonRepo(t *testing.T) {
	// A non-git directory must not panic or error; fields are just empty.
	g := Git(t.TempDir())
	if g.Commit != "" || g.Branch != "" || g.Dirty {
		t.Errorf("non-repo Git = %+v, want zero value", g)
	}
}

func TestGitStampKeys(t *testing.T) {
	dir := initRepo(t)
	bi, err := GitStamp("example.com/app/version")(&Context{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"Commit", "Short", "Branch", "Tag", "Time", "Dirty"} {
		if _, ok := bi.Vars["example.com/app/version."+k]; !ok {
			t.Errorf("missing -X var for %s", k)
		}
	}
	if bi.Vars["example.com/app/version.Branch"] != "main" {
		t.Errorf("Branch var = %q, want main", bi.Vars["example.com/app/version.Branch"])
	}
}

func TestGitEnvKeys(t *testing.T) {
	dir := initRepo(t)
	bi, err := GitEnv()(&Context{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if bi.Env["GW_GIT_BRANCH"] != "main" {
		t.Errorf("GW_GIT_BRANCH = %q, want main", bi.Env["GW_GIT_BRANCH"])
	}
	if len(bi.Env["GW_GIT_COMMIT"]) != 40 {
		t.Errorf("GW_GIT_COMMIT = %q, want a 40-char SHA", bi.Env["GW_GIT_COMMIT"])
	}
}
