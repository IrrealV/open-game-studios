package commands

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// mcpConfigFake builds one disposable workspace plus fake runtime paths. The
// fake executables record invocation in a sentinel file, so a test can prove
// the emitter never runs an interpreter or engine.
type mcpConfigFake struct {
	root           string
	sentinel       string
	bootstrap      string
	launcher       string
	venvPython     string
	blenderAdapter string
	godotNode      string
	godotIndex     string
	blenderEngine  string
	godotEngine    string
	blenderProject string
	godotProject   string
	blenderRuntime string
	godotRuntime   string
	blenderOutput  string
}

func newMCPConfigFake(t *testing.T) *mcpConfigFake {
	t.Helper()
	root := t.TempDir()
	f := &mcpConfigFake{root: root, sentinel: filepath.Join(root, "executed.marker")}
	f.bootstrap = f.writeExecutable(t, "bin/bootstrap-python")
	f.launcher = f.writeFile(t, "tools/mcp/ogs_mcp_launch.py")
	f.venvPython = f.writeExecutable(t, "tools/mcp/blender/venv/bin/python")
	f.blenderAdapter = f.writeFile(t, "tools/mcp/blender_headless.py")
	f.godotNode = f.writeExecutable(t, "tools/mcp/godot/bin/node")
	f.godotIndex = f.writeFile(t, "tools/mcp/godot/build/index.js")
	f.blenderEngine = f.writeExecutable(t, "engines/blender")
	f.godotEngine = f.writeExecutable(t, "engines/godot")
	f.blenderProject = f.mkdir(t, "projects/blender")
	f.godotProject = f.mkdir(t, "projects/godot")
	// The launcher preflights the full runtime child list and the Godot project
	// marker, so a valid fixture must already provide them. The emitter never
	// provisions these directories; tests that remove them prove it refuses.
	f.blenderRuntime = f.mkdirRuntime(t, "runtimes/blender")
	f.godotRuntime = f.mkdirRuntime(t, "runtimes/godot")
	f.blenderOutput = f.mkdir(t, "outputs/blender")
	f.writeFile(t, "projects/godot/project.godot")
	f.writeFile(t, ".game-studio/workspace.manifest.json")
	t.Chdir(root)
	return f
}

// mcpConfigTestRuntimeSubdirs mirrors the launcher's RUNTIME_SUBDIRS preflight so
// fixtures provide exactly the directories the launcher requires to exist.
var mcpConfigTestRuntimeSubdirs = []string{"home", "tmp", "config", "cache", "data", "state", "run"}

func (f *mcpConfigFake) path(rel string) string {
	return filepath.Join(f.root, filepath.FromSlash(rel))
}

func (f *mcpConfigFake) writeFile(t *testing.T, rel string) string {
	t.Helper()
	path := f.path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte("// fixture\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return path
}

func (f *mcpConfigFake) writeExecutable(t *testing.T, rel string) string {
	t.Helper()
	path := f.path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	script := "#!/bin/sh\nprintf invoked >> '" + f.sentinel + "'\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", rel, err)
	}
	return path
}

func (f *mcpConfigFake) mkdir(t *testing.T, rel string) string {
	t.Helper()
	path := f.path(rel)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	return path
}

// mkdirRuntime provisions a runtime root plus every child directory the
// launcher's static preflight requires.
func (f *mcpConfigFake) mkdirRuntime(t *testing.T, rel string) string {
	t.Helper()
	root := f.mkdir(t, rel)
	for _, name := range mcpConfigTestRuntimeSubdirs {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir runtime child %s/%s: %v", rel, name, err)
		}
	}
	return root
}

func (f *mcpConfigFake) blenderServer() map[string]any {
	return map[string]any{
		"name":         "ogs-blender",
		"mode":         "blender",
		"interpreter":  f.venvPython,
		"entrypoint":   f.blenderAdapter,
		"engine":       f.blenderEngine,
		"project_root": f.blenderProject,
		"runtime_root": f.blenderRuntime,
		"output_root":  f.blenderOutput,
	}
}

func (f *mcpConfigFake) godotServer() map[string]any {
	return map[string]any{
		"name":         "ogs-godot",
		"mode":         "godot",
		"interpreter":  f.godotNode,
		"entrypoint":   f.godotIndex,
		"engine":       f.godotEngine,
		"project_root": f.godotProject,
		"runtime_root": f.godotRuntime,
	}
}

func (f *mcpConfigFake) jointSpec() map[string]any {
	return map[string]any{
		"bootstrap_python": f.bootstrap,
		"launcher":         f.launcher,
		"servers":          []any{f.blenderServer(), f.godotServer()},
	}
}

func (f *mcpConfigFake) writeSpec(t *testing.T, spec map[string]any) string {
	t.Helper()
	raw, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	return f.writeRawSpec(t, append(raw, '\n'))
}

func (f *mcpConfigFake) writeRawSpec(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(f.root, "mcp-spec.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}

type mcpConfigReadEntry struct {
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
}

func readMCPConfigEntries(t *testing.T, path string) map[string]mcpConfigReadEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		MCPServers map[string]mcpConfigReadEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return doc.MCPServers
}

func assertNoMCPConfigStaging(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".mcp.json.") && strings.HasSuffix(name, ".tmp") {
			t.Fatalf("staging file left behind in %s: %s", dir, name)
		}
	}
}

func assertMCPConfigNotCreated(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, ".pi")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no .pi directory under %s, got err=%v", root, err)
	}
}

func assertMCPConfigMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode().Perm() != want {
		t.Fatalf("%s has mode %o, want %o", path, info.Mode().Perm(), want)
	}
}

func TestMCPConfigEmitsJointNativeConfig(t *testing.T) {
	f := newMCPConfigFake(t)
	specPath := f.writeSpec(t, f.jointSpec())

	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err != nil {
		t.Fatalf("RunMCPConfig: %v", err)
	}

	target := filepath.Join(f.root, ".pi", "mcp.json")
	entries := readMCPConfigEntries(t, target)
	if len(entries) != 2 {
		t.Fatalf("expected two servers, got %d: %#v", len(entries), entries)
	}

	blender, ok := entries["ogs-blender"]
	if !ok {
		t.Fatalf("missing ogs-blender entry: %#v", entries)
	}
	godot, ok := entries["ogs-godot"]
	if !ok {
		t.Fatalf("missing ogs-godot entry: %#v", entries)
	}

	wantBlender := []string{
		"-i", "PATH=/usr/bin:/bin",
		f.bootstrap, "-B", f.launcher, "blender",
		"--interpreter", f.venvPython,
		"--entrypoint", f.blenderAdapter,
		"--engine", f.blenderEngine,
		"--project-root", f.blenderProject,
		"--runtime-root", f.blenderRuntime,
		"--output-root", f.blenderOutput,
	}
	wantGodot := []string{
		"-i", "PATH=/usr/bin:/bin",
		f.bootstrap, "-B", f.launcher, "godot",
		"--interpreter", f.godotNode,
		"--entrypoint", f.godotIndex,
		"--engine", f.godotEngine,
		"--project-root", f.godotProject,
		"--runtime-root", f.godotRuntime,
	}
	if blender.Command != "/usr/bin/env" || godot.Command != "/usr/bin/env" {
		t.Fatalf("expected /usr/bin/env command, got %q and %q", blender.Command, godot.Command)
	}
	if !reflect.DeepEqual(blender.Args, wantBlender) {
		t.Fatalf("blender args mismatch\n got: %#v\nwant: %#v", blender.Args, wantBlender)
	}
	if !reflect.DeepEqual(godot.Args, wantGodot) {
		t.Fatalf("godot args mismatch\n got: %#v\nwant: %#v", godot.Args, wantGodot)
	}
	if blender.Enabled || godot.Enabled {
		t.Fatalf("entries must start disabled, got blender=%t godot=%t", blender.Enabled, godot.Enabled)
	}
	if !strings.Contains(blender.Description, "unsandboxed") {
		t.Fatalf("blender description must state the unsandboxed bpy trust boundary: %q", blender.Description)
	}
	if !strings.Contains(godot.Description, "projectPath") {
		t.Fatalf("godot description must state the per-call projectPath boundary: %q", godot.Description)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read raw target: %v", err)
	}
	if strings.Contains(string(raw), `"env"`) {
		t.Fatalf("configuration must not embed an env block: %s", raw)
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		t.Fatalf("decode outer document: %v", err)
	}
	if len(outer) != 1 {
		t.Fatalf("expected only mcpServers at top level, got %v", keysOf(outer))
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(outer["mcpServers"], &inner); err != nil {
		t.Fatalf("decode mcpServers: %v", err)
	}
	for name, entryRaw := range inner {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(entryRaw, &fields); err != nil {
			t.Fatalf("decode entry %s: %v", name, err)
		}
		if len(fields) != 4 {
			t.Fatalf("entry %s has unexpected fields %v", name, keysOf(fields))
		}
		for _, want := range []string{"command", "args", "description", "enabled"} {
			if _, ok := fields[want]; !ok {
				t.Fatalf("entry %s is missing %q", name, want)
			}
		}
	}

	assertMCPConfigMode(t, target, 0o600)
	assertMCPConfigMode(t, filepath.Join(f.root, ".pi"), 0o700)
	assertNoMCPConfigStaging(t, filepath.Join(f.root, ".pi"))

	piEntries, err := os.ReadDir(filepath.Join(f.root, ".pi"))
	if err != nil {
		t.Fatalf("read .pi: %v", err)
	}
	if len(piEntries) != 1 || piEntries[0].Name() != "mcp.json" {
		t.Fatalf("expected only mcp.json in .pi, got %v", piEntries)
	}

	if _, err := os.Stat(f.sentinel); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("emitter must never invoke an interpreter or engine, sentinel err=%v", err)
	}
}

func TestMCPConfigEmitsSingleGodotServer(t *testing.T) {
	f := newMCPConfigFake(t)
	spec := map[string]any{
		"bootstrap_python": f.bootstrap,
		"launcher":         f.launcher,
		"servers":          []any{f.godotServer()},
	}
	specPath := f.writeSpec(t, spec)

	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err != nil {
		t.Fatalf("RunMCPConfig: %v", err)
	}

	entries := readMCPConfigEntries(t, filepath.Join(f.root, ".pi", "mcp.json"))
	if len(entries) != 1 {
		t.Fatalf("expected one server, got %#v", entries)
	}
	godot := entries["ogs-godot"]
	for _, arg := range godot.Args {
		if arg == "--output-root" {
			t.Fatalf("godot entry must not pass --output-root: %#v", godot.Args)
		}
	}
}

func TestMCPConfigPreservesExistingDestination(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		f := newMCPConfigFake(t)
		dir := f.mkdir(t, ".pi")
		target := filepath.Join(dir, "mcp.json")
		if err := os.WriteFile(target, []byte{}, 0o600); err != nil {
			t.Fatalf("seed empty target: %v", err)
		}
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected refusal for an existing empty destination")
		}
		info, err := os.Stat(target)
		if err != nil || info.Size() != 0 {
			t.Fatalf("empty destination must be preserved untouched, info=%v err=%v", info, err)
		}
		assertNoMCPConfigStaging(t, dir)
	})

	t.Run("non-empty file", func(t *testing.T) {
		f := newMCPConfigFake(t)
		dir := f.mkdir(t, ".pi")
		target := filepath.Join(dir, "mcp.json")
		original := []byte("{\"mcpServers\":{\"human\":{}}}\n")
		if err := os.WriteFile(target, original, 0o600); err != nil {
			t.Fatalf("seed target: %v", err)
		}
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected refusal for an existing destination")
		}
		got, err := os.ReadFile(target)
		if err != nil || !reflect.DeepEqual(got, original) {
			t.Fatalf("destination must be preserved untouched, got=%q err=%v", got, err)
		}
		assertNoMCPConfigStaging(t, dir)
	})

	t.Run("directory", func(t *testing.T) {
		f := newMCPConfigFake(t)
		target := filepath.Join(f.mkdir(t, ".pi"), "mcp.json")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatalf("seed directory target: %v", err)
		}
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected refusal when the destination is a directory")
		}
		if info, err := os.Stat(target); err != nil || !info.IsDir() {
			t.Fatalf("directory destination must be untouched, info=%v err=%v", info, err)
		}
		assertNoMCPConfigStaging(t, filepath.Dir(target))
	})
}

func TestMCPConfigRefusesSymlinkedPiDirectory(t *testing.T) {
	f := newMCPConfigFake(t)
	realDir := f.mkdir(t, "real-pi")
	if err := os.Symlink(realDir, filepath.Join(f.root, ".pi")); err != nil {
		t.Fatalf("symlink .pi: %v", err)
	}
	specPath := f.writeSpec(t, f.jointSpec())
	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
		t.Fatal("expected refusal when .pi is a symlink")
	}
	if _, err := os.Stat(filepath.Join(realDir, "mcp.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("must not write through a symlinked .pi, err=%v", err)
	}
	assertNoMCPConfigStaging(t, realDir)
}

func TestMCPConfigRefusesSymlinkedTarget(t *testing.T) {
	f := newMCPConfigFake(t)
	dir := f.mkdir(t, ".pi")
	outside := f.writeFile(t, "outside.json")
	if err := os.Symlink(outside, filepath.Join(dir, "mcp.json")); err != nil {
		t.Fatalf("symlink target: %v", err)
	}
	specPath := f.writeSpec(t, f.jointSpec())
	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
		t.Fatal("expected refusal when the target is a symlink")
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "// fixture\n" {
		t.Fatalf("symlink must be left untouched, got=%q err=%v", got, err)
	}
	assertNoMCPConfigStaging(t, dir)
}

func TestMCPConfigRejectsInvalidServers(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(f *mcpConfigFake, spec map[string]any)
	}{
		{"invalid name characters", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["name"] = "ogs blender"
		}},
		{"empty name", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["name"] = ""
		}},
		{"duplicate name", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 1)["name"] = "ogs-blender"
		}},
		{"normalized collision", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 1)["name"] = "ogs_blender"
		}},
		{"unknown mode", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["mode"] = "unity"
		}},
		{"blender without output root", func(_ *mcpConfigFake, spec map[string]any) {
			delete(serverAt(spec, 0), "output_root")
		}},
		{"godot with output root", func(f *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 1)["output_root"] = f.blenderOutput
		}},
		{"no servers", func(_ *mcpConfigFake, spec map[string]any) {
			spec["servers"] = []any{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMCPConfigFake(t)
			spec := f.jointSpec()
			tc.mutate(f, spec)
			specPath := f.writeSpec(t, spec)
			if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
				t.Fatal("expected an invalid spec to be refused")
			}
			assertMCPConfigNotCreated(t, f.root)
		})
	}
}

func TestMCPConfigRejectsInvalidPaths(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(f *mcpConfigFake, spec map[string]any)
	}{
		{"relative bootstrap", func(_ *mcpConfigFake, spec map[string]any) {
			spec["bootstrap_python"] = "bin/bootstrap-python"
		}},
		{"missing launcher", func(_ *mcpConfigFake, spec map[string]any) {
			spec["launcher"] = "/nonexistent/ogs_mcp_launch.py"
		}},
		{"non-executable bootstrap", func(f *mcpConfigFake, spec map[string]any) {
			spec["bootstrap_python"] = f.launcher
		}},
		{"non-executable interpreter", func(f *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["interpreter"] = f.blenderAdapter
		}},
		{"non-executable engine", func(f *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["engine"] = f.godotIndex
		}},
		{"entrypoint is a directory", func(f *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["entrypoint"] = f.blenderProject
		}},
		{"project root is a file", func(f *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["project_root"] = f.launcher
		}},
		{"runtime root is missing", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["runtime_root"] = "/nonexistent/runtime"
		}},
		{"output root is a file", func(f *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["output_root"] = f.launcher
		}},
		{"relative project root", func(_ *mcpConfigFake, spec map[string]any) {
			serverAt(spec, 0)["project_root"] = "projects/blender"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMCPConfigFake(t)
			spec := f.jointSpec()
			tc.mutate(f, spec)
			specPath := f.writeSpec(t, spec)
			if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
				t.Fatal("expected an invalid path to be refused")
			}
			assertMCPConfigNotCreated(t, f.root)
		})
	}
}

func TestMCPConfigRejectsSymlinkedInputComponent(t *testing.T) {
	f := newMCPConfigFake(t)
	f.mkdir(t, "real-bin")
	f.writeExecutable(t, "real-bin/python")
	if err := os.Symlink(f.path("real-bin"), f.path("bin-link")); err != nil {
		t.Fatalf("symlink input component: %v", err)
	}
	spec := f.jointSpec()
	serverAt(spec, 0)["interpreter"] = f.path("bin-link/python")
	specPath := f.writeSpec(t, spec)
	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
		t.Fatal("expected a symlinked input component to be refused")
	}
	assertMCPConfigNotCreated(t, f.root)
}

func TestMCPConfigRefusesWithoutWorkspaceMarker(t *testing.T) {
	f := newMCPConfigFake(t)
	plain := t.TempDir()
	t.Chdir(plain)
	specPath := f.writeSpec(t, f.jointSpec())
	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
		t.Fatal("expected refusal when no workspace marker is present")
	}
	assertMCPConfigNotCreated(t, plain)
}

func TestMCPConfigRejectsUnknownSpecFields(t *testing.T) {
	t.Run("top-level", func(t *testing.T) {
		f := newMCPConfigFake(t)
		spec := f.jointSpec()
		spec["extra"] = true
		specPath := f.writeSpec(t, spec)
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected an unknown top-level field to be refused")
		}
		assertMCPConfigNotCreated(t, f.root)
	})
	t.Run("server-level", func(t *testing.T) {
		f := newMCPConfigFake(t)
		spec := f.jointSpec()
		serverAt(spec, 0)["extra"] = true
		specPath := f.writeSpec(t, spec)
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected an unknown server field to be refused")
		}
		assertMCPConfigNotCreated(t, f.root)
	})
}

func TestMCPConfigRejectsTrailingJSON(t *testing.T) {
	f := newMCPConfigFake(t)
	raw, err := json.Marshal(f.jointSpec())
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	for _, suffix := range []string{"\n{\"second\":1}\n", "\nnot json\n"} {
		t.Run(strings.TrimSpace(suffix), func(t *testing.T) {
			specPath := f.writeRawSpec(t, append(raw, []byte(suffix)...))
			if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
				t.Fatal("expected trailing JSON to be refused")
			}
			assertMCPConfigNotCreated(t, f.root)
		})
	}
}

func TestMCPConfigRejectsCLIArgErrors(t *testing.T) {
	f := newMCPConfigFake(t)
	specPath := f.writeSpec(t, f.jointSpec())
	cases := []struct {
		name string
		args []string
	}{
		{"missing spec flag", nil},
		{"unknown flag", []string{"--spec", specPath, "--out", "custom.json"}},
		{"trailing positional", []string{"--spec", specPath, "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := RunMCPConfig(MCPConfigInput{Args: tc.args}); err == nil {
				t.Fatal("expected CLI argument error")
			}
			assertMCPConfigNotCreated(t, f.root)
		})
	}
}

func TestMCPConfigPreservesExistingPiPermissions(t *testing.T) {
	f := newMCPConfigFake(t)
	dir := f.mkdir(t, ".pi")
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod .pi: %v", err)
	}
	specPath := f.writeSpec(t, f.jointSpec())
	if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err != nil {
		t.Fatalf("RunMCPConfig: %v", err)
	}
	assertMCPConfigMode(t, dir, 0o755)
	assertMCPConfigMode(t, filepath.Join(dir, "mcp.json"), 0o600)
}

func TestMCPConfigWriteExclusivePublishesNewTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mcp.json")
	data := []byte("{\"mcpServers\":{}}\n")
	if err := writeExclusiveMCPConfig(dir, target, data); err != nil {
		t.Fatalf("writeExclusiveMCPConfig: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(got, data) {
		t.Fatalf("published bytes mismatch, got=%q err=%v", got, err)
	}
	assertMCPConfigMode(t, target, 0o600)
	assertNoMCPConfigStaging(t, dir)
}

func TestMCPConfigWriteExclusiveRefusesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mcp.json")
	original := []byte("keep\n")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	err := writeExclusiveMCPConfig(dir, target, []byte("new\n"))
	if err == nil {
		t.Fatal("expected an existing target to be refused")
	}
	if !strings.Contains(err.Error(), target) {
		t.Fatalf("error must name the untouched target, got: %v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil || !reflect.DeepEqual(got, original) {
		t.Fatalf("existing target must be preserved, got=%q err=%v", got, readErr)
	}
	assertNoMCPConfigStaging(t, dir)
}

// TestMCPConfigRejectsIncompleteRuntimeRoots pins config-time validation to the
// launcher's LaunchPlan requirement: every runtime child directory the launched
// process will use must already exist as a real directory. The emitter must
// never create or repair them, and must leave .pi untouched on refusal.
func TestMCPConfigRejectsIncompleteRuntimeRoots(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, f *mcpConfigFake, spec map[string]any)
	}{
		{"missing runtime subdirectory", func(t *testing.T, f *mcpConfigFake, _ map[string]any) {
			if err := os.RemoveAll(filepath.Join(f.blenderRuntime, "tmp")); err != nil {
				t.Fatalf("remove runtime child: %v", err)
			}
		}},
		{"runtime subdirectory is a symlink", func(t *testing.T, f *mcpConfigFake, _ map[string]any) {
			child := filepath.Join(f.blenderRuntime, "cache")
			if err := os.RemoveAll(child); err != nil {
				t.Fatalf("remove runtime child: %v", err)
			}
			if err := os.Symlink(f.mkdir(t, "real-cache"), child); err != nil {
				t.Fatalf("symlink runtime child: %v", err)
			}
		}},
		{"runtime subdirectory is a file", func(t *testing.T, f *mcpConfigFake, _ map[string]any) {
			child := filepath.Join(f.blenderRuntime, "run")
			if err := os.RemoveAll(child); err != nil {
				t.Fatalf("remove runtime child: %v", err)
			}
			if err := os.WriteFile(child, []byte("not a directory\n"), 0o644); err != nil {
				t.Fatalf("write runtime child file: %v", err)
			}
		}},
		{"godot runtime subdirectory is missing", func(t *testing.T, f *mcpConfigFake, _ map[string]any) {
			if err := os.RemoveAll(filepath.Join(f.godotRuntime, "state")); err != nil {
				t.Fatalf("remove runtime child: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMCPConfigFake(t)
			spec := f.jointSpec()
			tc.mutate(t, f, spec)
			specPath := f.writeSpec(t, spec)
			if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
				t.Fatal("expected an incomplete runtime root to be refused before .pi is created")
			}
			assertMCPConfigNotCreated(t, f.root)
		})
	}
}

// TestMCPConfigRejectsGodotProjectMarkerProblems pins the Godot-only requirement
// that project_root contains a regular, non-symlinked project.godot marker.
func TestMCPConfigRejectsGodotProjectMarkerProblems(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, f *mcpConfigFake)
	}{
		{"missing project.godot", func(t *testing.T, f *mcpConfigFake) {
			if err := os.Remove(filepath.Join(f.godotProject, "project.godot")); err != nil {
				t.Fatalf("remove project.godot: %v", err)
			}
		}},
		{"project.godot is a symlink", func(t *testing.T, f *mcpConfigFake) {
			marker := filepath.Join(f.godotProject, "project.godot")
			if err := os.Remove(marker); err != nil {
				t.Fatalf("remove project.godot: %v", err)
			}
			if err := os.Symlink(f.writeFile(t, "real-project.godot"), marker); err != nil {
				t.Fatalf("symlink project.godot: %v", err)
			}
		}},
		{"project.godot is a directory", func(t *testing.T, f *mcpConfigFake) {
			marker := filepath.Join(f.godotProject, "project.godot")
			if err := os.Remove(marker); err != nil {
				t.Fatalf("remove project.godot: %v", err)
			}
			if err := os.Mkdir(marker, 0o755); err != nil {
				t.Fatalf("mkdir project.godot: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMCPConfigFake(t)
			tc.mutate(t, f)
			specPath := f.writeSpec(t, f.jointSpec())
			if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
				t.Fatal("expected a bad Godot project marker to be refused before .pi is created")
			}
			assertMCPConfigNotCreated(t, f.root)
		})
	}
}

// TestMCPConfigWorkspaceMarkerMustBeRegularFile proves workspace resolution only
// accepts a real regular marker inside the workspace, never a symlink or a
// directory that merely shares the marker name.
func TestMCPConfigWorkspaceMarkerMustBeRegularFile(t *testing.T) {
	markerPath := func(f *mcpConfigFake) string {
		return filepath.Join(f.root, ".game-studio", "workspace.manifest.json")
	}

	t.Run("symlinked marker is refused", func(t *testing.T) {
		f := newMCPConfigFake(t)
		marker := markerPath(f)
		if err := os.Remove(marker); err != nil {
			t.Fatalf("remove marker: %v", err)
		}
		if err := os.Symlink(f.writeFile(t, "real-marker.json"), marker); err != nil {
			t.Fatalf("symlink marker: %v", err)
		}
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected a symlinked workspace marker to be refused")
		}
		assertMCPConfigNotCreated(t, f.root)
	})

	t.Run("directory marker is refused", func(t *testing.T) {
		f := newMCPConfigFake(t)
		marker := markerPath(f)
		if err := os.Remove(marker); err != nil {
			t.Fatalf("remove marker: %v", err)
		}
		if err := os.Mkdir(marker, 0o755); err != nil {
			t.Fatalf("mkdir marker: %v", err)
		}
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected a directory workspace marker to be refused")
		}
		assertMCPConfigNotCreated(t, f.root)
	})

	t.Run("symlinked marker directory is refused", func(t *testing.T) {
		f := newMCPConfigFake(t)
		realDir := f.path("real-game-studio")
		if err := os.Rename(filepath.Join(f.root, ".game-studio"), realDir); err != nil {
			t.Fatalf("rename marker dir: %v", err)
		}
		if err := os.Symlink(realDir, filepath.Join(f.root, ".game-studio")); err != nil {
			t.Fatalf("symlink marker dir: %v", err)
		}
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("expected a symlinked workspace marker directory to be refused")
		}
		assertMCPConfigNotCreated(t, f.root)
	})

	t.Run("regular marker is accepted", func(t *testing.T) {
		f := newMCPConfigFake(t)
		specPath := f.writeSpec(t, f.jointSpec())
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err != nil {
			t.Fatalf("a valid regular workspace marker must be accepted: %v", err)
		}
		if _, err := os.Stat(filepath.Join(f.root, ".pi", "mcp.json")); err != nil {
			t.Fatalf("expected publication for a valid marker: %v", err)
		}
	})
}

// TestMCPConfigOutputRootNullSemantics makes the JSON null rule explicit. The
// launcher models a missing output root as None, so an explicit null is the same
// as omitting the field: allowed for Godot (no --output-root emitted) and still
// refused for Blender (which requires a real output root).
func TestMCPConfigOutputRootNullSemantics(t *testing.T) {
	t.Run("godot null is treated as omitted", func(t *testing.T) {
		f := newMCPConfigFake(t)
		server := f.godotServer()
		server["output_root"] = nil
		spec := map[string]any{
			"bootstrap_python": f.bootstrap,
			"launcher":         f.launcher,
			"servers":          []any{server},
		}
		specPath := f.writeSpec(t, spec)
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err != nil {
			t.Fatalf("godot output_root:null must be accepted as omitted: %v", err)
		}
		specRaw, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read spec: %v", err)
		}
		if !strings.Contains(string(specRaw), `"output_root": null`) {
			t.Fatalf("spec fixture must contain an explicit null output_root: %s", specRaw)
		}
		target := filepath.Join(f.root, ".pi", "mcp.json")
		for _, arg := range readMCPConfigEntries(t, target)["ogs-godot"].Args {
			if arg == "--output-root" {
				t.Fatalf("godot must not emit --output-root for a null output_root")
			}
		}
	})

	t.Run("blender null is refused", func(t *testing.T) {
		f := newMCPConfigFake(t)
		server := f.blenderServer()
		server["output_root"] = nil
		spec := map[string]any{
			"bootstrap_python": f.bootstrap,
			"launcher":         f.launcher,
			"servers":          []any{server},
		}
		specPath := f.writeSpec(t, spec)
		if err := RunMCPConfig(MCPConfigInput{Args: []string{"--spec", specPath}}); err == nil {
			t.Fatal("blender output_root:null must be refused because Blender requires an output root")
		}
		assertMCPConfigNotCreated(t, f.root)
	})
}

// TestMCPConfigWriteExclusiveCollisionIsDeterministic proves the publish step
// refuses a target that appears after the caller's own existence check, leaving
// the racing bytes untouched and no staging temp behind.
func TestMCPConfigWriteExclusiveCollisionIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mcp.json")
	racing := []byte("{\"mcpServers\":{\"racer\":{}}}\n")
	if err := os.WriteFile(target, racing, 0o600); err != nil {
		t.Fatalf("seed racing target: %v", err)
	}
	err := writeExclusiveMCPConfig(dir, target, []byte("{\"mcpServers\":{}}\n"))
	if err == nil {
		t.Fatal("expected exclusive publication to refuse a racing target")
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil || !reflect.DeepEqual(got, racing) {
		t.Fatalf("racing target must be preserved, got=%q err=%v", got, readErr)
	}
	assertNoMCPConfigStaging(t, dir)
}

// TestMCPConfigWriteExclusiveReportsStagingCreationFailure covers a bounded IO
// failure: when the staging directory cannot be used, the call fails without
// staging or publishing anything.
func TestMCPConfigWriteExclusiveReportsStagingCreationFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	err := writeExclusiveMCPConfig(dir, filepath.Join(dir, "mcp.json"), []byte("{}\n"))
	if err == nil {
		t.Fatal("expected a staging-creation failure when the directory does not exist")
	}
}

// TestMCPConfigWriteExclusiveReportsPublishCleanupFailure proves a cleanup
// failure after publication is reported as published-with-cleanup-failure and
// never rolls back the intact target. The removal seam is injected only inside
// this test.
func TestMCPConfigWriteExclusiveReportsPublishCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mcp.json")
	injected := errors.New("injected remove failure")
	originalRemove := mcpConfigRemoveFile
	t.Cleanup(func() { mcpConfigRemoveFile = originalRemove })
	mcpConfigRemoveFile = func(string) error { return injected }

	err := writeExclusiveMCPConfig(dir, target, []byte("{}\n"))
	if err == nil {
		t.Fatal("expected a cleanup failure to be reported")
	}
	var published *mcpConfigPublishedError
	if !errors.As(err, &published) {
		t.Fatalf("expected a published-with-cleanup-failure error, got %v", err)
	}
	if !errors.Is(err, injected) {
		t.Fatalf("cleanup failure must be wrapped, got %v", err)
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil || string(data) != "{}\n" {
		t.Fatalf("published target must stay intact when cleanup fails, data=%q err=%v", data, readErr)
	}
	entries, dirErr := os.ReadDir(dir)
	if dirErr != nil {
		t.Fatalf("read staging dir: %v", dirErr)
	}
	stagingLeft := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".mcp.json.") && strings.HasSuffix(entry.Name(), ".tmp") {
			stagingLeft = true
		}
	}
	if !stagingLeft {
		t.Fatal("an injected removal failure must leave the staging temp for an honest report")
	}
}

// TestMCPConfigWriteExclusiveReportsPrePublishCleanupFailure proves a cleanup
// failure on a pre-publish failure path (collision) is surfaced and the racing
// target is preserved.
func TestMCPConfigWriteExclusiveReportsPrePublishCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mcp.json")
	seed := []byte("seed\n")
	if err := os.WriteFile(target, seed, 0o600); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	originalRemove := mcpConfigRemoveFile
	t.Cleanup(func() { mcpConfigRemoveFile = originalRemove })
	mcpConfigRemoveFile = func(string) error { return errors.New("injected remove failure") }

	err := writeExclusiveMCPConfig(dir, target, []byte("new\n"))
	if err == nil {
		t.Fatal("expected a publish failure")
	}
	if !strings.Contains(err.Error(), "staging cleanup failed") {
		t.Fatalf("error must report the staging cleanup failure, got %v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil || !reflect.DeepEqual(got, seed) {
		t.Fatalf("racing target must be preserved, got=%q err=%v", got, readErr)
	}
}

func serverAt(spec map[string]any, index int) map[string]any {
	servers, ok := spec["servers"].([]any)
	if !ok || index >= len(servers) {
		panic("spec servers index out of range")
	}
	server, ok := servers[index].(map[string]any)
	if !ok {
		panic("spec server is not an object")
	}
	return server
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
