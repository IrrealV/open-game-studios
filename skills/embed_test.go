package skills

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"
)

// canonicalGroups is the expected hardwired group table. Keep it explicit so a
// change to the manifest fails here instead of passing silently.
var canonicalGroups = map[string][]string{
	"ogs-godot-change": {"SKILL.md", "references/godot-setup.md"},
	"ogs-core":         {"SKILL.md", "references/handoff-contract.md"},
}

func TestGroupsExposeCanonicalFiles(t *testing.T) {
	groups := Groups()
	if len(groups) != len(canonicalGroups) {
		t.Fatalf("Groups() returned %d groups, want %d", len(groups), len(canonicalGroups))
	}
	for _, group := range groups {
		want, ok := canonicalGroups[group.Dir]
		if !ok {
			t.Fatalf("unexpected embedded group %q", group.Dir)
		}
		if len(group.Files) != len(want) {
			t.Fatalf("group %q has %d files, want %d", group.Dir, len(group.Files), len(want))
		}
		for _, rel := range group.Files {
			data, err := ReadGroupFile(group.Dir, rel)
			if err != nil {
				t.Fatalf("ReadGroupFile(%q, %q) failed: %v", group.Dir, rel, err)
			}
			if len(data) == 0 {
				t.Errorf("embedded %s/%s is empty", group.Dir, rel)
			}
		}
	}

	coreSkill, err := ReadGroupFile("ogs-core", "SKILL.md")
	if err != nil {
		t.Fatalf("ReadGroupFile(ogs-core, SKILL.md) failed: %v", err)
	}
	if !bytes.HasPrefix(coreSkill, []byte("---")) {
		t.Errorf("ogs-core/SKILL.md does not start with frontmatter delimiter")
	}
	if !bytes.Contains(coreSkill, []byte("name: ogs-core")) {
		t.Errorf("ogs-core/SKILL.md does not declare the canonical skill name")
	}
	if !bytes.Contains(coreSkill, []byte("references/handoff-contract.md")) {
		t.Errorf("ogs-core/SKILL.md does not reference its local handoff contract")
	}

	legacySkill, err := ReadGroupFile("ogs-godot-change", "SKILL.md")
	if err != nil {
		t.Fatalf("ReadGroupFile(ogs-godot-change, SKILL.md) failed: %v", err)
	}
	if !bytes.Contains(legacySkill, []byte("name: ogs-godot-change")) {
		t.Errorf("ogs-godot-change/SKILL.md does not declare the canonical skill name")
	}

	handoff, err := ReadGroupFile("ogs-core", "references/handoff-contract.md")
	if err != nil {
		t.Fatalf("ReadGroupFile(ogs-core, references/handoff-contract.md) failed: %v", err)
	}
	if !bytes.Contains(handoff, []byte("OGS handoff contract")) {
		t.Errorf("handoff contract is missing the expected heading")
	}

	paths := RelativePaths()
	wantPaths := []string{
		"ogs-godot-change/SKILL.md",
		"ogs-godot-change/references/godot-setup.md",
		"ogs-core/SKILL.md",
		"ogs-core/references/handoff-contract.md",
	}
	if len(paths) != len(wantPaths) {
		t.Fatalf("RelativePaths() = %v, want %v", paths, wantPaths)
	}
	for index, want := range wantPaths {
		if paths[index] != want {
			t.Errorf("RelativePaths()[%d] = %q, want %q", index, paths[index], want)
		}
	}
}

func TestEmbeddedPayloadMatchesRepositoryFiles(t *testing.T) {
	for _, group := range Groups() {
		for _, rel := range group.Files {
			onDisk, err := os.ReadFile(filepath.Join(group.Dir, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("repository file %q is unreadable: %v", filepath.Join(group.Dir, rel), err)
			}
			embedded, err := ReadGroupFile(group.Dir, rel)
			if err != nil {
				t.Fatalf("ReadGroupFile(%q, %q) failed: %v", group.Dir, rel, err)
			}
			if !bytes.Equal(onDisk, embedded) {
				t.Errorf("embedded %s/%s differs from the canonical repository file", group.Dir, rel)
			}
		}
	}
}

func TestGroupsReturnsDeepCopy(t *testing.T) {
	first := Groups()
	first[0].Dir = "mutated"
	first[0].Files[0] = "mutated.md"
	first[0].Files = append(first[0].Files, "extra.md")

	second := Groups()
	if second[0].Dir == "mutated" {
		t.Errorf("mutating a returned group changed the manifest directory")
	}
	if second[0].Files[0] == "mutated.md" {
		t.Errorf("mutating a returned file list changed the manifest")
	}
	for _, file := range second[0].Files {
		if file == "extra.md" {
			t.Errorf("appending to a returned file list changed the manifest")
		}
	}
	if _, err := ReadGroupFile(second[0].Dir, second[0].Files[0]); err != nil {
		t.Errorf("manifest was corrupted by a returned-copy mutation: %v", err)
	}
}

func TestReadGroupFileRejectsUndeclaredGroupAndFile(t *testing.T) {
	cases := []struct {
		dir string
		rel string
	}{
		{"unknown", "SKILL.md"},
		{"", "SKILL.md"},
		{"ogs-core", ""},
		{"ogs-core", "../SKILL.md"},
		{"ogs-core", "references/"},
		{"ogs-core", "./SKILL.md"},
		{"ogs-core", "references/../SKILL.md"},
		{"ogs-core", "references/godot-setup.md"},
		{"ogs-godot-change", "references/handoff-contract.md"},
		{"ogs-godot-change", "ogs-core/SKILL.md"},
	}
	for _, tc := range cases {
		if _, err := ReadGroupFile(tc.dir, tc.rel); err == nil {
			t.Errorf("ReadGroupFile(%q, %q) succeeded, want rejection", tc.dir, tc.rel)
		}
	}
}

// TestEmbedManifestMatchesEmbeddedTree proves the go:embed directive and the
// group manifest stay coherent: every embedded file is declared, and every
// declared file is embedded (the read test above proves the second half).
func TestEmbedManifestMatchesEmbeddedTree(t *testing.T) {
	declared := map[string]bool{}
	for _, group := range Groups() {
		for _, rel := range group.Files {
			declared[path.Join(group.Dir, rel)] = true
		}
	}

	var embedded []string
	err := fs.WalkDir(FS, ".", func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			embedded = append(embedded, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded filesystem failed: %v", err)
	}
	sort.Strings(embedded)
	if len(embedded) != len(declared) {
		t.Fatalf("embedded files %v do not match the declared manifest %v", embedded, declared)
	}
	for _, p := range embedded {
		if !declared[p] {
			t.Errorf("embedded file %q is not declared in the group manifest", p)
		}
	}
}
