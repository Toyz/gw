package workspace

import (
	"fmt"
	"sort"
)

// placeholderPseudo is the version the go tool writes for a require that is only
// satisfied locally (by a workspace `use` or a `replace`), never by a real
// published version. Seeing it in a committed go.mod means the dependency has no
// release an external consumer could resolve.
const placeholderPseudo = "v0.0.0-00010101000000-000000000000"

// VerifyLevel ranks a finding.
type VerifyLevel string

const (
	LevelError VerifyLevel = "error"
	LevelWarn  VerifyLevel = "warn"
)

// Finding is one release-contract problem that workspace mode hides.
type Finding struct {
	Level   VerifyLevel `json:"level"`
	Code    string      `json:"code"`
	Module  string      `json:"module"`
	Dep     string      `json:"dep,omitempty"`
	Version string      `json:"version,omitempty"`
	Message string      `json:"message"`
}

// Release is one step of the release plan: a module whose code has moved past
// its latest published tag (or was never tagged), plus the workspace modules
// that will need a require bump and re-tag once it is released.
type Release struct {
	Module     string   `json:"module"`
	LatestTag  string   `json:"latestTag,omitempty"`
	Reason     string   `json:"reason"`
	Dependents []string `json:"dependents,omitempty"`
}

// VerifyReport is the full result of Verify.
type VerifyReport struct {
	Findings []Finding `json:"findings"`
	Releases []Release `json:"releases"`
}

// Errors counts error-level findings.
func (r VerifyReport) Errors() int { return r.count(LevelError) }

// Warnings counts warn-level findings.
func (r VerifyReport) Warnings() int { return r.count(LevelWarn) }

func (r VerifyReport) count(l VerifyLevel) int {
	n := 0
	for _, f := range r.Findings {
		if f.Level == l {
			n++
		}
	}
	return n
}

// Verify checks the release contract that workspace mode papers over: every
// require on another workspace module must point at a real published tag whose
// code matches what's on disk, and no module may leak a local-path replace.
// Workspace builds pass regardless of any of this — Verify runs the checks an
// external consumer (or a GOWORK=off release build) would actually hit.
//
// It also produces a release plan, in dependency order, for every module whose
// code has moved past its latest tag.
func Verify(g *Graph, mods []Module, gitRoot string) (VerifyReport, error) {
	var rep VerifyReport
	member := make(map[string]Module, len(mods))
	for _, m := range mods {
		member[m.Path] = m
	}

	sorted := append([]Module(nil), mods...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	// tag -> whether the depended module's subtree has changed since that tag.
	staleCache := map[string]bool{}

	for _, a := range sorted {
		if a.GoMod == nil {
			continue
		}

		// A local-path replace won't resolve for an external consumer: the path
		// simply doesn't exist outside this checkout. gw's model hoists these
		// into go.work; one lingering in a go.mod is a release blocker.
		for _, r := range a.GoMod.Replace {
			if !isLocalPath(r.New.Path) {
				continue
			}
			rep.Findings = append(rep.Findings, Finding{
				Level: LevelError, Code: "local-replace", Module: a.Path, Dep: r.Old.Path,
				Message: fmt.Sprintf("%s go.mod replaces %s => %s (local path); an external build can't resolve it — hoist it into go.work (gw init) or drop it before release",
					a.Path, r.Old.Path, r.New.Path),
			})
		}

		// Every require on another workspace member is a release contract.
		for _, r := range a.GoMod.Require {
			b, ok := member[r.Mod.Path]
			if !ok {
				continue // external dep — the proxy resolves it normally
			}
			ver := r.Mod.Version
			prefix := ModuleTagPrefix(gitRoot, b.Dir)

			if ver == "" || ver == placeholderPseudo {
				rep.Findings = append(rep.Findings, Finding{
					Level: LevelError, Code: "workspace-only", Module: a.Path, Dep: b.Path, Version: ver,
					Message: fmt.Sprintf("%s requires %s only via the workspace (no real version pinned); tag %s and pin it before %s can build or publish outside the workspace",
						a.Path, b.Path, prefix+"vX.Y.Z", a.Path),
				})
				continue
			}

			tag := prefix + ver
			if !TagExists(gitRoot, tag) {
				rep.Findings = append(rep.Findings, Finding{
					Level: LevelError, Code: "unpublished-require", Module: a.Path, Dep: b.Path, Version: ver,
					Message: fmt.Sprintf("%s requires %s %s but tag %q doesn't exist — an external build of %s would fail to resolve it (workspace mode hides this)",
						a.Path, b.Path, ver, tag, a.Path),
				})
				continue
			}

			stale, seen := staleCache[tag]
			if !seen {
				var err error
				stale, err = SubtreeChanged(gitRoot, tag, b, mods)
				if err != nil {
					return rep, fmt.Errorf("diffing %s against %s: %w", b.Path, tag, err)
				}
				staleCache[tag] = stale
			}
			if stale {
				rep.Findings = append(rep.Findings, Finding{
					Level: LevelWarn, Code: "stale-require", Module: a.Path, Dep: b.Path, Version: ver,
					Message: fmt.Sprintf("%s pins %s %s, but %s has changed since that tag — %s builds against newer local code while consumers get %s; re-tag %s and bump the require",
						a.Path, b.Path, ver, b.Path, a.Path, ver, b.Path),
				})
			}
		}
	}

	// Release plan: modules ahead of their latest tag (or never tagged), ordered
	// so dependencies come before the modules that require them.
	pos := make(map[string]int)
	for i, p := range g.TopoOrder() {
		pos[p] = i
	}
	for _, m := range sorted {
		var deps []string
		for _, d := range g.TransitiveDependents([]string{m.Path}) {
			if d != m.Path {
				deps = append(deps, d)
			}
		}
		tags, err := ModuleTags(gitRoot, m.Dir)
		if err != nil {
			return rep, fmt.Errorf("listing tags for %s: %w", m.Path, err)
		}
		rel := Release{Module: m.Path, Dependents: deps}
		if len(tags) == 0 {
			// A never-tagged module only needs releasing if a workspace member
			// depends on it — a leaf app that nobody imports needs no tag.
			if len(deps) == 0 {
				continue
			}
			rel.Reason = "never tagged"
		} else {
			rel.LatestTag = ModuleTagPrefix(gitRoot, m.Dir) + tags[0]
			changed, err := SubtreeChanged(gitRoot, rel.LatestTag, m, mods)
			if err != nil {
				return rep, fmt.Errorf("diffing %s against %s: %w", m.Path, rel.LatestTag, err)
			}
			if !changed {
				continue // released and clean — nothing to do
			}
			rel.Reason = "changed since " + rel.LatestTag
		}
		rep.Releases = append(rep.Releases, rel)
	}
	sort.Slice(rep.Releases, func(i, j int) bool {
		return pos[rep.Releases[i].Module] < pos[rep.Releases[j].Module]
	})

	return rep, nil
}
