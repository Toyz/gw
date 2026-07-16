package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestHookable pins the rules for which commands fire pre-/post- hooks: direct
// workspace subcommands do, meta verbs and the ext subtree don't, and a preview
// flag (--dry-run/--check) suppresses them.
func TestHookable(t *testing.T) {
	// A root named "gw" with a normal subcommand, an ext subtree, and a
	// subcommand carrying preview flags — mirroring the real tree's shape.
	root := &cobra.Command{Use: "gw"}

	build := &cobra.Command{Use: "build"}
	root.AddCommand(build)

	ext := &cobra.Command{Use: "ext"}
	extBuild := &cobra.Command{Use: "build"} // gw ext build — not gw build
	ext.AddCommand(extBuild)
	root.AddCommand(ext)

	sync := &cobra.Command{Use: "sync"}
	sync.Flags().Bool("dry-run", false, "")
	sync.Flags().Bool("check", false, "")
	root.AddCommand(sync)

	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string // raw argv (help detection)
		set  string   // flag to set true before the check, "" for none
		want bool
	}{
		{"direct subcommand", build, nil, "", true},
		{"sync no flags", sync, nil, "", true},
		{"root itself", root, nil, "", false},
		{"ext parent", ext, nil, "", false},
		{"ext subcommand", extBuild, nil, "", false},
		{"sync --dry-run suppressed", sync, nil, "dry-run", false},
		{"sync --check suppressed", sync, nil, "check", false},
		{"build --help skipped", build, []string{"--help"}, "", false},
		{"build -h skipped", build, []string{"-h"}, "", false},
		{"help after -- not skipped", build, []string{"--", "--help"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set != "" {
				if err := tt.cmd.Flags().Set(tt.set, "true"); err != nil {
					t.Fatalf("set %s: %v", tt.set, err)
				}
				defer tt.cmd.Flags().Set(tt.set, "false")
			}
			if got := hookable(tt.cmd, tt.args); got != tt.want {
				t.Errorf("hookable(%s, %v) = %v, want %v", tt.cmd.Name(), tt.args, got, tt.want)
			}
		})
	}
}
