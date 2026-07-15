package cmd

import (
	"reflect"
	"testing"
)

func TestSplitExecArgs(t *testing.T) {
	defer func() { rootFlag = "" }()
	tests := []struct {
		name     string
		args     []string
		wantRest []string
		parallel bool
		envVars  []string
		envFiles []string
		root     string
	}{
		{"go flags pass through", []string{"-v", "-run", "X", "./..."}, []string{"-v", "-run", "X", "./..."}, false, nil, nil, ""},
		{"gw flags stripped", []string{"-p", "--env", "A=1", "-race"}, []string{"-race"}, true, []string{"A=1"}, nil, ""},
		{"equals forms", []string{"--env=A=1", "--env-file=x.env"}, nil, false, []string{"A=1"}, []string{"x.env"}, ""},
		{"root captured", []string{"-C", "/w", "-v"}, []string{"-v"}, false, nil, nil, "/w"},
		{"dash-dash verbatim", []string{"-p", "--", "-p", "-v"}, []string{"-p", "-v"}, true, nil, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootFlag = ""
			f, rest := splitExecArgs(tt.args)
			if !reflect.DeepEqual(rest, tt.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
			}
			if f.parallel != tt.parallel {
				t.Errorf("parallel = %v, want %v", f.parallel, tt.parallel)
			}
			if !reflect.DeepEqual(f.envVars, tt.envVars) {
				t.Errorf("envVars = %v, want %v", f.envVars, tt.envVars)
			}
			if !reflect.DeepEqual(f.envFiles, tt.envFiles) {
				t.Errorf("envFiles = %v, want %v", f.envFiles, tt.envFiles)
			}
			if rootFlag != tt.root {
				t.Errorf("rootFlag = %q, want %q", rootFlag, tt.root)
			}
		})
	}
}

func TestWantsHelp(t *testing.T) {
	if !wantsHelp([]string{"-v", "--help"}) {
		t.Error("--help should be detected")
	}
	if wantsHelp([]string{"--", "--help"}) {
		t.Error("--help after -- should pass through, not trigger help")
	}
	if wantsHelp([]string{"-v", "./..."}) {
		t.Error("no help flag present")
	}
}
