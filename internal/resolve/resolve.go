// Package resolve turns a user's selection into an ordered, filtered list of
// tools to install.
//
// Everything here is pure: it takes a catalogue and a request and returns a
// plan of intent. No subprocesses, no network, no filesystem. That makes the
// hard parts — dependency ordering, conflict detection, honest handling of
// tools that cannot work on macOS — cheap to test exhaustively.
package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// SkipReason explains why a selected tool will not be installed.
type SkipReason string

const (
	// SkipConflict covers a tool excluded because something else in the
	// selection conflicts with it.
	SkipConflict SkipReason = "conflict"

	// SkipArch covers a tool that does not support this machine.
	SkipArch SkipReason = "unsupported-arch"

	// SkipUnsupportedBackend covers a tool whose backend is not built yet.
	SkipUnsupportedBackend SkipReason = "backend-not-implemented"
)

// Skipped is one excluded tool and the reason.
type Skipped struct {
	Tool   *manifest.Tool
	Reason SkipReason
	Detail string
}

// Result is a resolved selection.
type Result struct {
	// Install is the tools to install, in dependency order: anything a
	// tool `requires` appears before it.
	Install []*manifest.Tool

	// Skipped is everything excluded, with a reason for each. These are
	// reported to the user rather than silently dropped — quietly not
	// installing something the user asked for is the worst outcome.
	Skipped []Skipped
}

// Request is what the user asked for. Exactly one selector should be set.
type Request struct {
	Profile  string
	Category string
	Tools    []string

	// Arch is the target architecture; defaults to arm64.
	Arch string

	// SupportedBackends, when non-nil, filters out tools whose backend has
	// no implementation yet.
	SupportedBackends map[manifest.Backend]bool
}

// Resolve expands a request against the catalogue.
func Resolve(cat *manifest.Catalogue, req Request) (*Result, error) {
	arch := req.Arch
	if arch == "" {
		arch = manifest.ArchARM64
	}

	selected, err := selectTools(cat, req)
	if err != nil {
		return nil, err
	}

	res := &Result{}

	// Filter before ordering: an excluded tool should not drag its
	// dependencies in.
	var kept []*manifest.Tool
	for _, t := range selected {
		switch {
		case !t.SupportsArch(arch):
			res.Skipped = append(res.Skipped, Skipped{t, SkipArch,
				fmt.Sprintf("supports %s, this machine is %s", strings.Join(t.Arch, "/"), arch)})
		case req.SupportedBackends != nil && !req.SupportedBackends[t.Backend]:
			res.Skipped = append(res.Skipped, Skipped{t, SkipUnsupportedBackend,
				fmt.Sprintf("backend %q is not implemented yet", t.Backend)})
		default:
			kept = append(kept, t)
		}
	}

	kept, conflicts := dropConflicts(kept)
	res.Skipped = append(res.Skipped, conflicts...)

	ordered, err := topoSort(cat, kept)
	if err != nil {
		return nil, err
	}
	res.Install = ordered

	sort.SliceStable(res.Skipped, func(i, j int) bool {
		return res.Skipped[i].Tool.ID < res.Skipped[j].Tool.ID
	})
	return res, nil
}

// selectTools expands whichever selector the request carries into a
// deduplicated tool list, pulling in transitive `requires`.
func selectTools(cat *manifest.Catalogue, req Request) ([]*manifest.Tool, error) {
	seen := map[string]bool{}
	var out []*manifest.Tool

	add := func(id string) error {
		if seen[id] {
			return nil
		}
		t, ok := cat.Tool(id)
		if !ok {
			return fmt.Errorf("unknown tool %q", id)
		}
		seen[id] = true
		out = append(out, t)
		return nil
	}

	switch {
	case req.Profile != "":
		ids, err := expandProfile(cat, req.Profile, nil)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if err := add(id); err != nil {
				return nil, err
			}
		}

	case req.Category != "":
		tools := cat.InCategory(req.Category)
		if len(tools) == 0 {
			return nil, fmt.Errorf("unknown or empty category %q — known: %s",
				req.Category, strings.Join(cat.Categories(), ", "))
		}
		for _, t := range tools {
			if err := add(t.ID); err != nil {
				return nil, err
			}
		}

	case len(req.Tools) > 0:
		for _, id := range req.Tools {
			if err := add(id); err != nil {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("nothing selected: pass --profile, --category or --tool")
	}

	// Pull in dependencies transitively. A tool the user did not name but
	// which is required is still installed, and appears in the plan.
	for i := 0; i < len(out); i++ {
		for _, dep := range out[i].Requires {
			if err := add(dep); err != nil {
				return nil, fmt.Errorf("%s: %w", out[i].ID, err)
			}
		}
	}
	return out, nil
}

// expandProfile flattens a profile and everything it includes. The stack
// parameter guards against cycles, which validate should have caught already
// but which must not hang the installer regardless.
func expandProfile(cat *manifest.Catalogue, id string, stack []string) ([]string, error) {
	for _, s := range stack {
		if s == id {
			return nil, fmt.Errorf("profile include cycle: %s", strings.Join(append(stack, id), " → "))
		}
	}
	p, ok := cat.Profile(id)
	if !ok {
		var known []string
		for _, pr := range cat.Profiles {
			known = append(known, pr.ID)
		}
		sort.Strings(known)
		return nil, fmt.Errorf("unknown profile %q — known: %s", id, strings.Join(known, ", "))
	}

	var ids []string
	for _, inc := range p.Includes {
		sub, err := expandProfile(cat, inc, append(stack, id))
		if err != nil {
			return nil, err
		}
		ids = append(ids, sub...)
	}
	return append(ids, p.Tools...), nil
}

// dropConflicts removes tools that conflict with something else in the
// selection. The earlier tool wins, which makes the outcome depend on manifest
// and profile order rather than on map iteration — deterministic, and
// explainable to the user.
func dropConflicts(tools []*manifest.Tool) ([]*manifest.Tool, []Skipped) {
	var kept []*manifest.Tool
	var skipped []Skipped
	keptIDs := map[string]bool{}

	for _, t := range tools {
		conflict := ""
		for _, other := range t.ConflictsWith {
			if keptIDs[other] {
				conflict = other
				break
			}
		}
		if conflict == "" {
			// Also check the reverse direction: an already-kept tool
			// may declare the conflict instead of this one.
			for _, k := range kept {
				for _, c := range k.ConflictsWith {
					if c == t.ID {
						conflict = k.ID
						break
					}
				}
			}
		}

		if conflict != "" {
			skipped = append(skipped, Skipped{t, SkipConflict,
				"conflicts with " + conflict + ", which was selected first"})
			continue
		}
		kept = append(kept, t)
		keptIDs[t.ID] = true
	}
	return kept, skipped
}

// topoSort orders tools so dependencies install first. Only edges between
// tools actually in the selection are considered.
func topoSort(cat *manifest.Catalogue, tools []*manifest.Tool) ([]*manifest.Tool, error) {
	inSet := map[string]*manifest.Tool{}
	for _, t := range tools {
		inSet[t.ID] = t
	}

	const (
		unvisited = 0
		active    = 1
		done      = 2
	)
	state := map[string]int{}
	var out []*manifest.Tool

	var visit func(t *manifest.Tool, stack []string) error
	visit = func(t *manifest.Tool, stack []string) error {
		switch state[t.ID] {
		case done:
			return nil
		case active:
			return fmt.Errorf("dependency cycle: %s", strings.Join(append(stack, t.ID), " → "))
		}
		state[t.ID] = active
		for _, dep := range t.Requires {
			if d, ok := inSet[dep]; ok {
				if err := visit(d, append(stack, t.ID)); err != nil {
					return err
				}
			}
		}
		state[t.ID] = done
		out = append(out, t)
		return nil
	}

	for _, t := range tools {
		if err := visit(t, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}
