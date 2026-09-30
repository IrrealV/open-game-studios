package toolcheck

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const GodotToolID = "godot"

type GodotCheck struct{}

func NewGodotCheck() GodotCheck { return GodotCheck{} }

func (GodotCheck) Metadata() Metadata {
	return Metadata{ID: GodotToolID, Name: "Godot", Aliases: []string{"godot4"}, Commands: []string{"godot --version", "godot4 --version"}, Severity: SeverityRequired, Required: true, Remediation: "Install Godot 4 and ensure godot or godot4 is available on PATH. On Windows, reopen the shell after editing PATH."}
}

func (c GodotCheck) Run(ctx context.Context, runner CommandRunner) Result {
	meta := c.Metadata()
	attempted := make([]string, 0, len(meta.Commands))
	diagnostics := make([]string, 0, len(meta.Commands))
	foundRunnableIncomplete := false
	foundInvalidVersion := false
	for _, name := range []string{"godot", "godot4"} {
		attempted = append(attempted, name+" --version")
		path, err := runner.LookPath(name)
		if err != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("%s lookup: %v", name, err))
			continue
		}
		result, err := runner.Run(ctx, name, "--version")
		attempted[len(attempted)-1] = fmt.Sprintf("%s --version (%s)", name, path)
		if err != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("%s run: %v", name, err))
			continue
		}
		version := strings.TrimSpace(result.Stdout)
		if version == "" {
			version = strings.TrimSpace(result.Stderr)
		}
		if version == "" {
			foundRunnableIncomplete = true
			diagnostics = append(diagnostics, fmt.Sprintf("%s version: Godot binary is runnable but did not print a version", name))
			continue
		}
		if isGodot4Version(version) {
			return resultForMetadata(meta, StatusSuccess, "detected "+firstLine(version), attempted[len(attempted)-1])
		}
		if godotMajorVersion(version) != "" {
			foundInvalidVersion = true
			diagnostics = append(diagnostics, fmt.Sprintf("%s version: Godot 4 is required; detected %s", name, firstLine(version)))
			continue
		}
		foundInvalidVersion = true
		diagnostics = append(diagnostics, fmt.Sprintf("%s version: Godot 4 is required; could not parse version output: %s", name, firstLine(version)))
	}
	reason := "Godot executable was not found or could not run"
	if len(diagnostics) > 0 {
		reason = strings.Join(diagnostics, "; ")
	}
	if foundRunnableIncomplete && !foundInvalidVersion {
		return resultForMetadata(meta, StatusWarning, reason, strings.Join(attempted, ", "))
	}
	return resultForMetadata(meta, StatusFailure, reason, strings.Join(attempted, ", "))
}

var godotVersionPattern = regexp.MustCompile(`(?i)(?:godot(?: engine)?\s*)?v?([0-9]+)(?:\.[0-9]+)?`)

func isGodot4Version(version string) bool {
	return godotMajorVersion(version) == "4"
}

func godotMajorVersion(version string) string {
	matches := godotVersionPattern.FindStringSubmatch(version)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func resultForMetadata(meta Metadata, status Status, reason, attempted string) Result {
	return Result{ToolID: meta.ID, ToolName: meta.Name, Status: status, Severity: meta.Severity, Required: meta.Required, Reason: reason, Attempted: attempted, Remediation: meta.Remediation}
}
