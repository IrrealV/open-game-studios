package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// mcpConfigMaxSpecBytes bounds the local JSON spec read by `mcp-config`.
const mcpConfigMaxSpecBytes = 1 << 20

const (
	mcpConfigDirName  = ".pi"
	mcpConfigFileName = "mcp.json"

	mcpConfigBlenderDescription = "Local Blender MCP adapter for this project: trusted bpy code runs unsandboxed inside the project; output_root supplies HOME/XDG/tmp, not artifact confinement. Explicit absolute paths only."
	mcpConfigGodotDescription   = "Local Godot MCP runtime for this project: upstream tools accept a projectPath per call, so the working directory does not confine them; use a dedicated session, not shared routing. Explicit absolute paths only."
)

// mcpConfigWorkspaceMarkers mirror detectWorkspaceRoot's accepted anchors, but
// resolution refuses instead of falling back to the caller's cwd.
var mcpConfigWorkspaceMarkers = []string{
	filepath.Join(".game-studio", "workspace.manifest.json"),
	workspaceConfigRelativePath,
	filepath.Join("openspec", "config.yaml"),
}

// mcpConfigRuntimeSubdirs mirrors the launcher's RUNTIME_SUBDIRS preflight. The
// launcher validates every child before exec, so the emitter must refuse a
// runtime root that is missing any of them at configuration time. The emitter
// never creates these directories.
var mcpConfigRuntimeSubdirs = []string{"home", "tmp", "config", "cache", "data", "state", "run"}

// mcpConfigRemoveFile removes one path and defaults to os.Remove. It is a narrow
// test seam so a test can prove a staging-temp cleanup failure is reported
// instead of being silently treated as clean. Production code never replaces it.
var mcpConfigRemoveFile = os.Remove

// mcpConfigPublishedError reports that the target was published while a later
// cleanup step failed. It never authorizes removing the published target.
type mcpConfigPublishedError struct{ err error }

func (e *mcpConfigPublishedError) Error() string { return e.err.Error() }

func (e *mcpConfigPublishedError) Unwrap() error { return e.err }

// MCPConfigInput is the narrowed input seam for `game-studio mcp-config`.
// Stdout is injectable for tests; a nil writer falls back to os.Stdout.
type MCPConfigInput struct {
	Args   []string
	Stdout io.Writer
}

type mcpConfigSpec struct {
	BootstrapPython string                `json:"bootstrap_python"`
	Launcher        string                `json:"launcher"`
	Servers         []mcpConfigServerSpec `json:"servers"`
}

type mcpConfigServerSpec struct {
	Name        string  `json:"name"`
	Mode        string  `json:"mode"`
	Interpreter string  `json:"interpreter"`
	Entrypoint  string  `json:"entrypoint"`
	Engine      string  `json:"engine"`
	ProjectRoot string  `json:"project_root"`
	RuntimeRoot string  `json:"runtime_root"`
	OutputRoot  *string `json:"output_root"`
	Description string  `json:"description"`
}

type mcpServerEntry struct {
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
}

type mcpConfigDocument struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// RunMCPConfig emits one create-only Pi project MCP configuration for the
// explicit local runtimes named in a caller-supplied JSON spec. It never
// discovers runtimes, expands environment variables, merges into an existing
// file, or starts Pi, MCP servers, interpreters, or engines.
func RunMCPConfig(input MCPConfigInput) error {
	stdout := input.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	set := flag.NewFlagSet("mcp-config", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	specPath := set.String("spec", "", "path to a local JSON spec (required)")
	if err := set.Parse(input.Args); err != nil {
		return fmt.Errorf("mcp-config: %w\n\n%s", err, mcpConfigUsage())
	}
	if set.NArg() != 0 {
		return fmt.Errorf("mcp-config: unexpected positional arguments %v\n\n%s", set.Args(), mcpConfigUsage())
	}
	if strings.TrimSpace(*specPath) == "" {
		return fmt.Errorf("mcp-config: --spec is required\n\n%s", mcpConfigUsage())
	}

	spec, resolvedSpecPath, err := readMCPConfigSpec(*specPath)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("mcp-config: resolve working directory: %w", err)
	}
	workspaceRoot, err := resolveMCPConfigWorkspaceRoot(cwd)
	if err != nil {
		return err
	}

	document, err := buildMCPConfig(spec)
	if err != nil {
		return err
	}

	target, err := publishMCPConfig(workspaceRoot, document)
	if err != nil {
		return err
	}

	return writeReport(stdout, []string{
		fmt.Sprintf("[mcp-config] wrote: %q\n", target),
		fmt.Sprintf("[mcp-config] spec: %q\n", resolvedSpecPath),
		fmt.Sprintf("[mcp-config] servers: %d\n", len(document.MCPServers)),
		"[mcp-config] every entry is enabled: false; no environment variables or credentials are embedded.\n",
		"[mcp-config] note: static project configuration only; this command starts no Pi session, MCP server, interpreter, or engine.\n",
	})
}

// readMCPConfigSpec reads one bounded JSON spec from an explicit path and
// rejects unknown fields, a second value, and trailing data. The file is only
// read; its bytes are never modified.
func readMCPConfigSpec(value string) (mcpConfigSpec, string, error) {
	abs, err := filepath.Abs(value)
	if err != nil {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: resolve spec path %q: %w", value, err)
	}
	if err := rejectSymlinkedComponents(abs, "spec file"); err != nil {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: read spec %q: %w", abs, err)
	}
	if info.IsDir() {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: spec %q is a directory, expected a JSON file", abs)
	}
	if !info.Mode().IsRegular() {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: spec %q is not a regular file", abs)
	}

	file, err := os.Open(abs)
	if err != nil {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: open spec %q: %w", abs, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, mcpConfigMaxSpecBytes+1))
	if err != nil {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: read spec %q: %w", abs, err)
	}
	if len(data) > mcpConfigMaxSpecBytes {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: spec %q exceeds the %d-byte input limit", abs, mcpConfigMaxSpecBytes)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var spec mcpConfigSpec
	if err := decoder.Decode(&spec); err != nil {
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: decode spec %q: %w", abs, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: spec %q contains more than one JSON value", abs)
		}
		return mcpConfigSpec{}, "", fmt.Errorf("mcp-config: spec %q has trailing data after the JSON value: %w", abs, err)
	}
	return spec, abs, nil
}

// resolveMCPConfigWorkspaceRoot walks up from start looking for an existing
// workspace marker. Unlike detectWorkspaceRoot it never falls back to start:
// when no marker exists the command refuses instead of guessing a root. A marker
// must be a real regular file inside the workspace, so a symlinked or directory
// marker is refused rather than silently resolved through.
func resolveMCPConfigWorkspaceRoot(start string) (string, error) {
	current := filepath.Clean(start)
	for {
		for _, marker := range mcpConfigWorkspaceMarkers {
			path := filepath.Join(current, marker)
			info, err := os.Lstat(path)
			switch {
			case errors.Is(err, os.ErrNotExist):
				continue
			case err != nil:
				return "", fmt.Errorf("mcp-config: inspect workspace marker %q: %w", path, err)
			case info.Mode()&os.ModeSymlink != 0:
				return "", fmt.Errorf("mcp-config: workspace marker %q is a symlink; refusing to resolve a workspace through it", path)
			case !info.Mode().IsRegular():
				return "", fmt.Errorf("mcp-config: workspace marker %q is not a regular file; refusing", path)
			}
			if err := rejectSymlinkedComponents(path, "workspace marker"); err != nil {
				return "", fmt.Errorf("mcp-config: %w", err)
			}
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("mcp-config: no Game-Studio workspace marker found from %q; run inside a project containing .game-studio/workspace.manifest.json, .game-studio/workspace.config.json, or openspec/config.yaml", start)
		}
		current = parent
	}
}

// buildMCPConfig validates every explicit path and returns the complete Pi
// mcpServers document. All requested servers are emitted together in one
// configuration; there is no per-entry merge with an existing file.
func buildMCPConfig(spec mcpConfigSpec) (mcpConfigDocument, error) {
	if len(spec.Servers) == 0 {
		return mcpConfigDocument{}, fmt.Errorf("mcp-config: spec must define at least one server")
	}
	bootstrapPython, err := validateMCPExecutableFile(spec.BootstrapPython, "bootstrap_python")
	if err != nil {
		return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
	}
	launcher, err := validateMCPReadableFile(spec.Launcher, "launcher")
	if err != nil {
		return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
	}

	servers := make(map[string]mcpServerEntry, len(spec.Servers))
	namespaces := make(map[string]string, len(spec.Servers))
	for _, server := range spec.Servers {
		label := fmt.Sprintf("server %q", server.Name)
		if err := validateMCPServerName(server.Name); err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}
		if _, exists := servers[server.Name]; exists {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: duplicate server name %q", server.Name)
		}
		namespace := strings.ReplaceAll(server.Name, "-", "_")
		if other, exists := namespaces[namespace]; exists {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: server names %q and %q normalize to the same Pi tool namespace %q; rename one", other, server.Name, namespace)
		}
		namespaces[namespace] = server.Name

		switch server.Mode {
		case "blender", "godot":
		default:
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %s has unsupported mode %q; expected blender or godot", label, server.Mode)
		}

		interpreter, err := validateMCPExecutableFile(server.Interpreter, label+" interpreter")
		if err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}
		entrypoint, err := validateMCPReadableFile(server.Entrypoint, label+" entrypoint")
		if err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}
		engine, err := validateMCPExecutableFile(server.Engine, label+" engine")
		if err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}
		projectRoot, err := validateMCPDirectory(server.ProjectRoot, label+" project_root")
		if err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}
		runtimeRoot, err := validateMCPDirectory(server.RuntimeRoot, label+" runtime_root")
		if err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}
		if err := validateMCPRuntimeChildren(runtimeRoot, label); err != nil {
			return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
		}

		args := []string{
			"-i", "PATH=/usr/bin:/bin",
			bootstrapPython, "-B", launcher,
			server.Mode,
			"--interpreter", interpreter,
			"--entrypoint", entrypoint,
			"--engine", engine,
			"--project-root", projectRoot,
			"--runtime-root", runtimeRoot,
		}
		description := strings.TrimSpace(server.Description)
		switch server.Mode {
		case "blender":
			if server.OutputRoot == nil || strings.TrimSpace(*server.OutputRoot) == "" {
				return mcpConfigDocument{}, fmt.Errorf("mcp-config: %s (blender) requires output_root", label)
			}
			outputRoot, err := validateMCPDirectory(*server.OutputRoot, label+" output_root")
			if err != nil {
				return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
			}
			args = append(args, "--output-root", outputRoot)
			if description == "" {
				description = mcpConfigBlenderDescription
			}
		case "godot":
			if server.OutputRoot != nil {
				return mcpConfigDocument{}, fmt.Errorf("mcp-config: %s (godot) takes no output_root; it is Blender-only", label)
			}
			if _, err := validateMCPRegularFile(filepath.Join(projectRoot, "project.godot"), label+" project.godot"); err != nil {
				return mcpConfigDocument{}, fmt.Errorf("mcp-config: %w", err)
			}
			if description == "" {
				description = mcpConfigGodotDescription
			}
		}

		servers[server.Name] = mcpServerEntry{
			Command:     "/usr/bin/env",
			Args:        args,
			Description: description,
			Enabled:     false,
		}
	}

	return mcpConfigDocument{MCPServers: servers}, nil
}

func validateMCPServerName(name string) error {
	if name == "" {
		return fmt.Errorf("server name must not be empty")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		allowed := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
		if !allowed {
			return fmt.Errorf("server name %q contains %q; allowed characters are letters, digits, '_' and '-'", name, string(c))
		}
	}
	return nil
}

func validateMCPAbsolute(value, label string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s must not be empty", label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%s must be an absolute path, got %q", label, value)
	}
	clean := filepath.Clean(trimmed)
	if err := rejectSymlinkedComponents(clean, label); err != nil {
		return "", err
	}
	return clean, nil
}

func validateMCPExecutableFile(value, label string) (string, error) {
	path, err := validateMCPAbsolute(value, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s is missing: %s", label, path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not an existing regular file: %s", label, path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable: %s", label, path)
	}
	return path, nil
}

func validateMCPReadableFile(value, label string) (string, error) {
	path, err := validateMCPAbsolute(value, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s is missing: %s", label, path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not an existing regular file: %s", label, path)
	}
	if info.Mode().Perm()&0o444 == 0 {
		return "", fmt.Errorf("%s is not readable: %s", label, path)
	}
	return path, nil
}

func validateMCPDirectory(value, label string) (string, error) {
	path, err := validateMCPAbsolute(value, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s is missing: %s", label, path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not an existing directory: %s", label, path)
	}
	return path, nil
}

// validateMCPRegularFile matches the launcher's plain file preflight: a real
// regular file with no symlinked component. It deliberately does not require a
// permission bit, mirroring LaunchPlan rather than inventing a stricter rule.
func validateMCPRegularFile(value, label string) (string, error) {
	path, err := validateMCPAbsolute(value, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s is missing: %s", label, path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not an existing regular file: %s", label, path)
	}
	return path, nil
}

// validateMCPRuntimeChildren mirrors the launcher's runtime preflight: every
// child directory the launched process will use must already exist as a real
// directory. The emitter never creates or repairs them.
func validateMCPRuntimeChildren(runtimeRoot, label string) error {
	for _, name := range mcpConfigRuntimeSubdirs {
		if _, err := validateMCPDirectory(filepath.Join(runtimeRoot, name), fmt.Sprintf("%s runtime %q directory", label, name)); err != nil {
			return err
		}
	}
	return nil
}

// rejectSymlinkedComponents refuses any symlinked component of path, so a
// supplied or discovered path can never be followed through a link. Every
// component must exist; use it only for paths that must already exist.
func rejectSymlinkedComponents(path, label string) error {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("%s must be an absolute path, got %q", label, path)
	}
	current := string(filepath.Separator)
	remainder := strings.TrimPrefix(clean, string(filepath.Separator))
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%s is missing: %s", label, current)
			}
			return fmt.Errorf("inspect %s component %s: %w", label, current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s has a symlinked component: %s", label, current)
		}
	}
	return nil
}

// publishMCPConfig creates the fixed WORKSPACE/.pi/mcp.json target. It refuses
// any existing target (file, symlink, or directory) untouched, creates a new
// .pi with 0700, and leaves an existing real .pi unchanged.
func publishMCPConfig(workspaceRoot string, document mcpConfigDocument) (string, error) {
	if err := rejectSymlinkedComponents(workspaceRoot, "workspace root"); err != nil {
		return "", fmt.Errorf("mcp-config: %w", err)
	}
	dir := filepath.Join(workspaceRoot, mcpConfigDirName)
	target := filepath.Join(dir, mcpConfigFileName)

	dirInfo, dirErr := os.Lstat(dir)
	switch {
	case dirErr == nil && dirInfo.Mode()&os.ModeSymlink != 0:
		return "", fmt.Errorf("mcp-config: refuse to use %q: it is a symlink", dir)
	case dirErr == nil && !dirInfo.IsDir():
		return "", fmt.Errorf("mcp-config: refuse to use %q: it is not a directory", dir)
	case dirErr != nil && !errors.Is(dirErr, os.ErrNotExist):
		return "", fmt.Errorf("mcp-config: inspect %q: %w", dir, dirErr)
	}

	if targetInfo, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("mcp-config: refuse to write %q: it already exists (%s); remove it yourself if you intend to regenerate", target, mcpConfigFileKind(targetInfo))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("mcp-config: inspect %q: %w", target, err)
	}

	if errors.Is(dirErr, os.ErrNotExist) {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return "", fmt.Errorf("mcp-config: create %q: %w", dir, err)
		}
	}

	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", fmt.Errorf("mcp-config: encode configuration: %w", err)
	}
	raw = append(raw, '\n')

	if err := writeExclusiveMCPConfig(dir, target, raw); err != nil {
		return target, err
	}
	return target, nil
}

func mcpConfigFileKind(info os.FileInfo) string {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case info.IsDir():
		return "directory"
	default:
		return "file"
	}
}

// writeExclusiveMCPConfig stages data in its own 0600 temp file inside dir and
// publishes it with an exclusive hard link, so it can never overwrite a target
// that appeared concurrently. Only its own staging temp is ever removed; a
// pre-existing or racing target is left untouched. A cleanup failure is always
// reported, and a cleanup failure after a successful publish is reported as
// mcpConfigPublishedError without ever rolling back the published target.
func writeExclusiveMCPConfig(dir, target string, data []byte) error {
	temp, err := os.CreateTemp(dir, ".mcp.json.*.tmp")
	if err != nil {
		return fmt.Errorf("mcp-config: create staging file in %q: %w", dir, err)
	}
	tempPath := temp.Name()
	removeStaging := func() error {
		if err := mcpConfigRemoveFile(tempPath); err != nil {
			return fmt.Errorf("remove its own staging file %q: %w", tempPath, err)
		}
		return nil
	}

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("mcp-config: write staging file %q: %w%s", tempPath, err, mcpConfigCleanupSuffix(removeStaging))
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("mcp-config: sync staging file %q: %w%s", tempPath, err, mcpConfigCleanupSuffix(removeStaging))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("mcp-config: close staging file %q: %w%s", tempPath, err, mcpConfigCleanupSuffix(removeStaging))
	}
	if err := os.Link(tempPath, target); err != nil {
		return fmt.Errorf("mcp-config: publish %q: %w (the target was left untouched)%s", target, err, mcpConfigCleanupSuffix(removeStaging))
	}
	if err := removeStaging(); err != nil {
		return &mcpConfigPublishedError{err: fmt.Errorf("mcp-config: published %q, but %w; the published target is intact and is never rolled back", target, err)}
	}
	return nil
}

// mcpConfigCleanupSuffix removes the caller's own staging temp and, when that
// removal fails, returns an honest suffix instead of pretending cleanup was
// clean. It never touches the published target.
func mcpConfigCleanupSuffix(removeStaging func() error) string {
	if err := removeStaging(); err != nil {
		return fmt.Sprintf(" (additionally, staging cleanup failed: %v)", err)
	}
	return ""
}

func mcpConfigUsage() string {
	return `mcp-config: emit one Pi project MCP configuration from an explicit JSON spec

Usage:
  game-studio mcp-config --spec <spec.json>

The spec field shape is documented in docs/mcp-config.md. Every path must be
absolute and explicit: no environment expansion, PATH lookup, or version
inference. Every requested server is emitted together into
WORKSPACE/.pi/mcp.json with enabled: false and no credentials or environment
block. The output target is create-only: an existing .pi/mcp.json, even empty,
is refused and never merged or overwritten. This command never starts Pi, an MCP
server, an interpreter, or an engine, and it never validates a runtime by
executing it.`
}
