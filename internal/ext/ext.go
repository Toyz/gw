// Package ext compiles and drives a workspace's optional .gw/build.go extension.
package ext

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/toyz/gw/gwext"
)

const (
	dirName   = ".gw"
	buildFile = "build.go"
	sentinel  = "__gwext"
)

// Dir returns the extension directory (.gw) inside root.
func Dir(root string) string { return filepath.Join(root, dirName) }

// Exists reports whether root has a .gw/build.go extension.
func Exists(root string) bool {
	_, err := os.Stat(filepath.Join(Dir(root), buildFile))
	return err == nil
}

// Build compiles .gw into a cached binary and returns its path. The binary is
// keyed by a hash of every source file under .gw (excluding bin/), so an
// unchanged extension is never rebuilt.
func Build(root string) (string, error) {
	gwDir := Dir(root)
	hash, err := hashDir(gwDir)
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(gwDir, "bin")
	binPath := filepath.Join(binDir, "ext-"+hash[:16])
	if isExecutable(binPath) {
		return binPath, nil
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = gwDir
	// The .gw module lives inside the workspace but is not a workspace member;
	// disable go.work so the build resolves against .gw/go.mod alone.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("building %s: %v\n%s", filepath.Join(dirName, buildFile), err, stderr.String())
	}
	return binPath, nil
}

// Manifest builds the extension and returns its registered commands and hooks.
func Manifest(root string) (gwext.Manifest, error) {
	var m gwext.Manifest
	bin, err := Build(root)
	if err != nil {
		return m, err
	}
	out, err := exec.Command(bin, sentinel, "manifest").Output()
	if err != nil {
		return m, fmt.Errorf("reading extension manifest: %w", err)
	}
	if err := json.Unmarshal(out, &m); err != nil {
		return m, fmt.Errorf("parsing extension manifest: %w", err)
	}
	return m, nil
}

// RunCommand builds the extension and runs a custom command, streaming stdio.
func RunCommand(root string, mods []gwext.Module, name string, args []string, stdout, stderr io.Writer) error {
	bin, err := Build(root)
	if err != nil {
		return err
	}
	argv := append([]string{sentinel, "command", name}, args...)
	return run(bin, argv, env(root, mods, ""), stdout, stderr)
}

// RunHook builds the extension (if present) and fires an event. It is a no-op
// when no extension exists. Hook failures are returned so callers can surface them.
func RunHook(root, event string, mods []gwext.Module, stdout, stderr io.Writer) error {
	if !Exists(root) {
		return nil
	}
	bin, err := Build(root)
	if err != nil {
		return err
	}
	return run(bin, []string{sentinel, "hook", event}, env(root, mods, event), stdout, stderr)
}

func run(bin string, argv []string, environ []string, stdout, stderr io.Writer) error {
	cmd := exec.Command(bin, argv...)
	cmd.Env = environ
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func env(root string, mods []gwext.Module, event string) []string {
	e := append(os.Environ(), "GW_ROOT="+root)
	if data, err := json.Marshal(mods); err == nil {
		e = append(e, "GW_MODULES="+string(data))
	}
	if event != "" {
		e = append(e, "GW_EVENT="+event)
	}
	return e
}

// hashDir returns a hex SHA-256 over the sorted (relpath, content) of every file
// under dir, skipping the bin/ cache subtree.
func hashDir(dir string) (string, error) {
	type file struct {
		rel  string
		data []byte
	}
	var files []file
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "bin" && path != dir {
				return fs.SkipDir
			}
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, file{filepath.ToSlash(rel), data})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%d\x00", f.rel, len(f.data))
		h.Write(f.data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
