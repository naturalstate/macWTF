package manifest

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Problem is a single validation failure, located precisely enough that a
// contributor can fix it without hunting.
type Problem struct {
	File string // manifest file the problem lives in
	Ref  string // tool or profile id, where one applies
	Msg  string
}

func (p Problem) Error() string {
	switch {
	case p.File != "" && p.Ref != "":
		return fmt.Sprintf("%s: %s: %s", p.File, p.Ref, p.Msg)
	case p.File != "":
		return fmt.Sprintf("%s: %s", p.File, p.Msg)
	default:
		return p.Msg
	}
}

// Problems is the full set of failures from one validation pass.
type Problems []Problem

func (ps Problems) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d validation problem(s):", len(ps))
	for _, p := range ps {
		b.WriteString("\n  " + p.Error())
	}
	return b.String()
}

var idPattern = regexp.MustCompile(`^[a-z0-9]+(?:[-.][a-z0-9]+)*$`)

// Validate checks schema conformance and referential integrity across the whole
// catalogue. It makes no network calls and must remain offline-safe.
//
// Every check runs against every entry: the goal is one complete list of
// problems, not a bail-out on the first. Package names resolving against real
// registries is a separate, online concern — see `macwtf check`.
func (c *Catalogue) Validate() error {
	var ps Problems

	ps = append(ps, c.validateTools()...)
	ps = append(ps, c.validateProfiles()...)

	if len(ps) == 0 {
		return nil
	}
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].File != ps[j].File {
			return ps[i].File < ps[j].File
		}
		return ps[i].Ref < ps[j].Ref
	})
	return ps
}

func (c *Catalogue) validateTools() Problems {
	var ps Problems
	seen := map[string]string{} // id -> first file it appeared in

	for _, t := range c.Tools {
		bad := func(format string, args ...any) {
			ps = append(ps, Problem{File: t.SourceFile, Ref: t.ID, Msg: fmt.Sprintf(format, args...)})
		}

		// Identity. Ids are permanent and end up in user state files, so
		// they are held to a strict shape.
		switch {
		case t.ID == "":
			ps = append(ps, Problem{File: t.SourceFile, Msg: "tool with empty id"})
		case !idPattern.MatchString(t.ID):
			bad("id must be lowercase kebab-case (a-z, 0-9, - and .)")
		}
		if first, dup := seen[t.ID]; dup && t.ID != "" {
			bad("duplicate id, already defined in %s", first)
		} else if t.ID != "" {
			seen[t.ID] = t.SourceFile
		}

		if t.Name == "" {
			bad("missing name")
		}
		if t.Description == "" {
			bad("missing description")
		}

		// Category must match the filename, so that the manifest directory
		// stays browsable and a tool cannot hide in the wrong file.
		if t.Category == "" {
			bad("missing category")
		} else if want := strings.TrimSuffix(path.Base(t.SourceFile), ".toml"); t.Category != want {
			bad("category %q does not match filename (expected %q)", t.Category, want)
		}

		// Backend and package.
		switch {
		case t.Backend == "":
			bad("missing backend")
		case !t.Backend.Valid():
			bad("unknown backend %q", t.Backend)
		case !t.Backend.ValidForMacOS():
			bad("backend %q cannot run on macOS; it belongs in a sibling platform block, not [tool.macos]", t.Backend)
		}
		needsPackage := t.Backend != BackendBuiltin && t.Backend != BackendManual && t.Backend != BackendDefaults
		if needsPackage && t.Package == "" {
			bad("backend %q requires a package", t.Backend)
		}
		if t.Tap != "" && t.Backend != BackendBrew && t.Backend != BackendCask {
			bad("tap is only meaningful for brew and cask backends")
		}

		// Architecture.
		for _, a := range t.Arch {
			if a != ArchARM64 && a != ArchAMD64 {
				bad("unknown arch %q (want %q or %q)", a, ArchARM64, ArchAMD64)
			}
		}
		if t.RequiresRosetta && t.SupportsArch(ArchARM64) && len(t.Arch) > 0 {
			bad("requires_rosetta is set but arch includes %q", ArchARM64)
		}

		// Permissions and manual steps.
		for _, p := range t.TCCPermissions {
			if !p.Valid() {
				bad("unknown tcc permission %q", p)
			}
		}
		if !t.License.Valid() {
			bad("unknown license %q", t.License)
		}

		// Quarantine stripping needs a concrete target, otherwise the
		// post-install step has nothing to act on.
		if t.QuarantineStrip && t.AppPath == "" {
			bad("quarantine_strip is set but app_path is empty; nothing to strip")
		}

		// Referential integrity.
		for _, dep := range t.Requires {
			if _, ok := c.byID[dep]; !ok {
				bad("requires unknown tool %q", dep)
			}
			if dep == t.ID {
				bad("requires itself")
			}
		}
		for _, cf := range t.ConflictsWith {
			if _, ok := c.byID[cf]; !ok {
				bad("conflicts_with unknown tool %q", cf)
			}
			if cf == t.ID {
				bad("conflicts with itself")
			}
		}
	}

	ps = append(ps, c.validateRequireCycles()...)
	return ps
}

// validateRequireCycles catches dependency loops, which would otherwise hang or
// misorder the installer.
func (c *Catalogue) validateRequireCycles() Problems {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var ps Problems

	var visit func(id string, stack []string)
	visit = func(id string, stack []string) {
		switch color[id] {
		case black:
			return
		case grey:
			ps = append(ps, Problem{
				Ref: id,
				Msg: "dependency cycle: " + strings.Join(append(stack, id), " → "),
			})
			return
		}
		color[id] = grey
		if t, ok := c.byID[id]; ok {
			for _, dep := range t.Requires {
				if _, exists := c.byID[dep]; exists {
					visit(dep, append(stack, id))
				}
			}
		}
		color[id] = black
	}

	ids := make([]string, 0, len(c.byID))
	for id := range c.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id, nil)
	}
	return ps
}

func (c *Catalogue) validateProfiles() Problems {
	var ps Problems
	seen := map[string]string{}

	for _, p := range c.Profiles {
		bad := func(format string, args ...any) {
			ps = append(ps, Problem{File: p.SourceFile, Ref: p.ID, Msg: fmt.Sprintf(format, args...)})
		}

		switch {
		case p.ID == "":
			ps = append(ps, Problem{File: p.SourceFile, Msg: "profile with empty id"})
		case !idPattern.MatchString(p.ID):
			bad("id must be lowercase kebab-case")
		}
		if first, dup := seen[p.ID]; dup && p.ID != "" {
			bad("duplicate profile id, already defined in %s", first)
		} else if p.ID != "" {
			seen[p.ID] = p.SourceFile
		}

		if p.Name == "" {
			bad("missing name")
		}
		if len(p.Tools) == 0 && len(p.Includes) == 0 {
			bad("profile is empty: no tools and no includes")
		}

		for _, id := range p.Tools {
			if _, ok := c.byID[id]; !ok {
				bad("references unknown tool %q", id)
			}
		}
		for _, inc := range p.Includes {
			if _, ok := c.profileByID[inc]; !ok {
				bad("includes unknown profile %q", inc)
			}
			if inc == p.ID {
				bad("includes itself")
			}
		}
	}

	ps = append(ps, c.validateProfileCycles()...)
	return ps
}

func (c *Catalogue) validateProfileCycles() Problems {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var ps Problems

	var visit func(id string, stack []string)
	visit = func(id string, stack []string) {
		switch color[id] {
		case black:
			return
		case grey:
			ps = append(ps, Problem{
				Ref: id,
				Msg: "profile include cycle: " + strings.Join(append(stack, id), " → "),
			})
			return
		}
		color[id] = grey
		if p, ok := c.profileByID[id]; ok {
			for _, inc := range p.Includes {
				if _, exists := c.profileByID[inc]; exists {
					visit(inc, append(stack, id))
				}
			}
		}
		color[id] = black
	}

	ids := make([]string, 0, len(c.profileByID))
	for id := range c.profileByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id, nil)
	}
	return ps
}
