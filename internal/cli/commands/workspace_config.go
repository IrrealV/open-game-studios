package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	workspaceConfigSchemaVersion = "workspace-config/v1"
	workspaceConfigRelativePath  = ".game-studio/workspace.config.json"

	presetSourceExplicitCLI        = "explicit_cli"
	presetSourcePersistedWorkspace = "persisted_workspace"
	presetSourceDefault            = "default"
	routingPresetSourceDerived     = "derived_from_setup_preset"
)

type workspaceConfig struct {
	SchemaVersion string `json:"schema_version"`
	SetupPreset   string `json:"setup_preset"`
	UpdatedAt     string `json:"updated_at"`
}

type resolvedPreset struct {
	SetupPreset         string
	PresetSource        string
	RoutingPreset       string
	RoutingPresetSource string
}

func newWorkspaceConfig(setupPreset string, updatedAt time.Time) (workspaceConfig, error) {
	config := workspaceConfig{
		SchemaVersion: workspaceConfigSchemaVersion,
		SetupPreset:   setupPreset,
		UpdatedAt:     updatedAt.UTC().Format(time.RFC3339),
	}
	if err := validateWorkspaceConfig(config); err != nil {
		return workspaceConfig{}, err
	}
	return config, nil
}

func readWorkspaceConfig(workspaceRoot string) (workspaceConfig, bool, error) {
	path := filepath.Join(workspaceRoot, filepath.FromSlash(workspaceConfigRelativePath))
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workspaceConfig{}, false, nil
		}
		return workspaceConfig{}, false, fmt.Errorf("read workspace config %s: %w", path, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var config workspaceConfig
	if err := decoder.Decode(&config); err != nil {
		return workspaceConfig{}, false, fmt.Errorf("workspace config %s is malformed or contains unexpected fields: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return workspaceConfig{}, false, fmt.Errorf("workspace config %s is malformed: %w", path, err)
	}
	if err := validateWorkspaceConfig(config); err != nil {
		return workspaceConfig{}, false, fmt.Errorf("workspace config %s is invalid: %w", path, err)
	}
	return config, true, nil
}

func writeWorkspaceConfigAtomic(workspaceRoot string, config workspaceConfig) error {
	if err := validateWorkspaceConfig(config); err != nil {
		return fmt.Errorf("refuse to write invalid workspace config: %w", err)
	}

	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace config: %w", err)
	}
	raw = append(raw, '\n')

	path := filepath.Join(workspaceRoot, filepath.FromSlash(workspaceConfigRelativePath))
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create workspace config directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".workspace.config.*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary workspace config: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return fmt.Errorf("set temporary workspace config permissions: %w", err)
	}
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary workspace config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary workspace config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary workspace config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace workspace config atomically: %w", err)
	}
	return nil
}

func validateWorkspaceConfig(config workspaceConfig) error {
	if config.SchemaVersion != workspaceConfigSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q: expected %q", config.SchemaVersion, workspaceConfigSchemaVersion)
	}
	if err := validateSetupPreset(config.SetupPreset); err != nil {
		return err
	}
	if config.UpdatedAt == "" {
		return fmt.Errorf("missing required updated_at")
	}
	if _, err := time.Parse(time.RFC3339, config.UpdatedAt); err != nil {
		return fmt.Errorf("invalid updated_at %q: expected RFC3339 timestamp", config.UpdatedAt)
	}
	return nil
}

func validateSetupPreset(value string) error {
	switch value {
	case "minimal", "recommended", "full", "custom":
		return nil
	case "":
		return fmt.Errorf("missing required setup_preset")
	default:
		return fmt.Errorf("invalid setup_preset %q: expected minimal, recommended, full, or custom", value)
	}
}

func resolveGeneratePreset(workspaceRoot, explicitPreset string, explicit bool) (resolvedPreset, error) {
	config, exists, err := readWorkspaceConfig(workspaceRoot)
	if err != nil {
		return resolvedPreset{}, err
	}

	if explicit {
		if err := validateSetupPreset(explicitPreset); err != nil {
			return resolvedPreset{}, fmt.Errorf("invalid --setup-depth: %w", err)
		}
		return newResolvedPreset(explicitPreset, presetSourceExplicitCLI), nil
	}

	if exists {
		return newResolvedPreset(config.SetupPreset, presetSourcePersistedWorkspace), nil
	}
	return newResolvedPreset("recommended", presetSourceDefault), nil
}

func newResolvedPreset(setupPreset, source string) resolvedPreset {
	return resolvedPreset{
		SetupPreset:         setupPreset,
		PresetSource:        source,
		RoutingPreset:       setupPreset,
		RoutingPresetSource: routingPresetSourceDerived,
	}
}
