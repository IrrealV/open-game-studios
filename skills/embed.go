// Package skills embeds the canonical Open Game Studios Pi skills so the Go
// installer can materialize them in a workspace without a developer checkout
// and without duplicating the canonical Markdown source.
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
)

// SkillsRoot is the workspace-relative directory the installer materializes the
// canonical skills into. It is hardwired: this package declares one small fixed
// skill set and deliberately offers no arbitrary skill directory, agent, or root
// routing.
const SkillsRoot = ".pi/skills"

// Group is one embedded canonical skill directory below SkillsRoot.
type Group struct {
	// Dir is the skill directory name below SkillsRoot.
	Dir string
	// Files are the group's payload-relative files, in payload order.
	Files []string
}

//go:embed ogs-godot-change/SKILL.md ogs-godot-change/references/godot-setup.md ogs-core/SKILL.md ogs-core/references/handoff-contract.md
var FS embed.FS

var groups = []Group{
	{Dir: "ogs-godot-change", Files: []string{"SKILL.md", "references/godot-setup.md"}},
	{Dir: "ogs-core", Files: []string{"SKILL.md", "references/handoff-contract.md"}},
}

// Groups returns a deep copy of the embedded skill group table. The caller may
// modify the returned slice and its nested file lists without affecting later
// calls or the embedded manifest.
func Groups() []Group {
	clone := make([]Group, len(groups))
	for index, group := range groups {
		clone[index] = Group{Dir: group.Dir, Files: append([]string(nil), group.Files...)}
	}
	return clone
}

// RelativePaths returns every embedded payload file as "skillDir/rel" across all
// groups, in group and payload order. The returned slice is a copy.
func RelativePaths() []string {
	paths := make([]string, 0, 4)
	for _, group := range groups {
		for _, rel := range group.Files {
			paths = append(paths, group.Dir+"/"+rel)
		}
	}
	return paths
}

// ReadGroupFile returns the embedded bytes for a declared skill directory and
// one of its payload-relative files. Only a declared group and file is read;
// any other group, file, or non-clean relative path is rejected before touching
// the embedded filesystem.
func ReadGroupFile(dir, rel string) ([]byte, error) {
	for _, group := range groups {
		if group.Dir != dir {
			continue
		}
		clean := path.Clean(rel)
		if clean != rel || !groupHasFile(group, clean) {
			return nil, fmt.Errorf("skills: %q is not part of the embedded %s skill", rel, dir)
		}
		return fs.ReadFile(FS, path.Join(dir, clean))
	}
	return nil, fmt.Errorf("skills: %q is not an embedded skill directory", dir)
}

func groupHasFile(group Group, rel string) bool {
	for _, file := range group.Files {
		if file == rel {
			return true
		}
	}
	return false
}
