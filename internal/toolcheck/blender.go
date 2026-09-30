package toolcheck

import (
	"context"
	"fmt"
	"strings"
)

const BlenderToolID = "blender"

type BlenderCheck struct{}

func NewBlenderCheck() BlenderCheck { return BlenderCheck{} }

func (BlenderCheck) Metadata() Metadata {
	return Metadata{ID: BlenderToolID, Name: "Blender", Commands: []string{"blender --version"}, Severity: SeverityRequired, Required: true, Remediation: "Install Blender and ensure blender is available on PATH. On Windows, add Blender's install directory to PATH and reopen PowerShell or Command Prompt."}
}

func (c BlenderCheck) Run(ctx context.Context, runner CommandRunner) Result {
	meta := c.Metadata()
	path, err := runner.LookPath("blender")
	attempted := "blender --version"
	if err != nil {
		return resultForMetadata(meta, StatusFailure, err.Error(), attempted)
	}
	attempted = fmt.Sprintf("blender --version (%s)", path)
	result, err := runner.Run(ctx, "blender", "--version")
	if err != nil {
		return resultForMetadata(meta, StatusFailure, err.Error(), attempted)
	}
	version := strings.TrimSpace(result.Stdout)
	if version == "" {
		version = strings.TrimSpace(result.Stderr)
	}
	if version == "" || !strings.Contains(strings.ToLower(version), "blender") {
		return resultForMetadata(meta, StatusWarning, "Blender binary is runnable but version output is incomplete", attempted)
	}
	return resultForMetadata(meta, StatusSuccess, "detected "+firstLine(version), attempted)
}

func firstLine(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\r\n", "\n")
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}
