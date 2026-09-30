package piinstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	ogsskills "open-game-studios/skills"
)

// planBinding is the normalized configuration that determines where execution
// writes and which explicit binaries it reuses. It is part of the approved
// fingerprint so a caller cannot approve one destination and execute another.
type planBinding struct {
	WorkspaceDir  string
	InstallRoot   string
	GodotRequired bool
	NodePath      string
	NpmPath       string
	PiPath        string
	EngramPath    string
	GodotPath     string
}

// planBinding normalizes the execution-relevant config with the same helper as
// Config.withDefaults, so the approved binding and every IO path agree. Paths
// are cleaned and explicit paths trimmed so trailing separators or whitespace do
// not produce a spurious drift refusal.
func (c Config) planBinding() planBinding {
	return planBinding{
		WorkspaceDir:  normalizeConfigPath(c.WorkspaceDir),
		InstallRoot:   normalizeConfigPath(c.InstallRoot),
		GodotRequired: c.GodotRequired,
		NodePath:      normalizeConfigPath(c.NodePath),
		NpmPath:       normalizeConfigPath(c.NpmPath),
		PiPath:        normalizeConfigPath(c.PiPath),
		EngramPath:    normalizeConfigPath(c.EngramPath),
		GodotPath:     normalizeConfigPath(c.GodotPath),
	}
}

// Prepare runs the read-only detection and builds the proposal without any
// host writes. It is the primary entry point for the wizard.
func Prepare(ctx context.Context, cfg Config) (Detection, Plan, error) {
	detection, err := Detect(ctx, cfg)
	if err != nil {
		return Detection{}, Plan{}, err
	}
	return detection, BuildPlan(cfg, detection), nil
}

// BuildPlan derives immutable conditional steps from a detection snapshot. It
// performs no host writes and no mutating commands.
func BuildPlan(cfg Config, detection Detection) Plan {
	cfg = cfg.withDefaults()
	plan := Plan{GodotRequired: cfg.GodotRequired, binding: cfg.planBinding()}

	nodeState := stateOrAbsent(detection, ComponentNode)
	npmState := stateOrAbsent(detection, ComponentNpm)
	piState := stateOrAbsent(detection, ComponentPi)
	engramState := stateOrAbsent(detection, ComponentEngramCore)
	payloadState := stateOrAbsent(detection, ComponentOGSPayload)

	nodeInstalling := planToolStep(&plan, cfg, nodeState, toolStepOptions{
		archiveName: "node",
		installEffects: []string{
			"downloads the official Node tar.gz over https and verifies its pinned SHA-256 before extraction",
			"extracts into an owned per-user install root; no PATH, shell-profile, system-prefix, or sudo changes",
			"archive size is not pinned; the digest is the integrity check",
		},
	})

	npmUsable := planNpmStep(&plan, cfg, nodeState, npmState, nodeInstalling)
	piUsable := planPiStep(&plan, cfg, piState, npmUsable)

	planEnsurePackageStep(&plan, cfg, detection, ComponentShell, "gentle-pi", ShellPackageSource, piUsable, []string{
		"registers the package with `pi install npm:gentle-pi@3.7.0 --no-approve` (personal scope)",
		"initializes Pi bootstrap settings as a side effect of the package command",
		"Gentle Shell's postinstall manages its package-private native runtime and can persist personal tuiMode fullscreen",
		"npm transitive dependency effects are not pinned by the package spec alone",
		"does not run standalone gentle-shell setup, --link, TUI, or gentle-ai install/sync",
	})

	planArchiveStep(&plan, cfg, engramState, archiveStepOptions{
		binaryName:  "engram",
		version:     "2.2.0",
		installKind: "engram core",
		installEffects: []string{
			"downloads the official Engram tar.gz over https and verifies its pinned SHA-256 and size before extraction",
			"extracts only the core binary into an owned per-user root; no PATH or shell-profile edits",
			"returns ENGRAM_BIN and a PATH addition for later real Engram use; no Engram process is called here",
		},
	})

	planEnsurePackageStep(&plan, cfg, detection, ComponentEngramCompanion, "gentle-engram", EngramCompanionPackageSource, piUsable, []string{
		"registers the package with `pi install npm:gentle-engram@0.1.15 --no-approve` (personal scope)",
		"initializes Pi bootstrap settings as a side effect of the package command",
		"requires the Engram core binary through PATH or ENGRAM_BIN; package installation alone is not MCP setup",
		"does not run pi-engram init, an optional MCP adapter, or model configuration",
	})

	if cfg.GodotRequired {
		planArchiveStep(&plan, cfg, stateOrAbsent(detection, ComponentGodot), archiveStepOptions{
			binaryName:  "Godot_v4.7.2-stable_linux.x86_64",
			version:     "4.7.2",
			installKind: "Godot 4.7.2",
			installEffects: []string{
				"downloads the official Godot zip over https and verifies its pinned SHA-256 and size before extraction",
				"extracts into an owned per-user root and returns GODOT_BIN; no PATH or shell-profile edits",
				"does not run gameplay or auto-approve any mechanic execution",
			},
		})
	}

	planPayloadStep(&plan, cfg, payloadState)
	return plan
}

type toolStepOptions struct {
	archiveName    string
	installEffects []string
}

func planToolStep(plan *Plan, cfg Config, state ComponentState, options toolStepOptions) bool {
	switch state.Compatibility {
	case CompatCompatible:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: state.Component,
			Action:    ActionReuse,
			Reason:    fmt.Sprintf("reuse compatible %s %s", state.Component, state.Version),
			Observed:  state,
		})
		return false
	case CompatAbsent:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: state.Component,
			Action:    ActionInstall,
			Reason:    fmt.Sprintf("%s is missing; install the pinned candidate", state.Component),
			Outputs:   []string{filepath.Join(cfg.InstallRoot, options.archiveName, versionFor(state.Component))},
			Effects:   options.installEffects,
			Command:   fmt.Sprintf("download+verify+extract %s", state.Component),
			Observed:  state,
		})
		return true
	default:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: state.Component,
			Action:    ActionBlocked,
			Reason:    blockedReason(state),
			Observed:  state,
		})
		return false
	}
}

func planNpmStep(plan *Plan, cfg Config, nodeState, npmState ComponentState, nodeInstalling bool) bool {
	managedNpm := filepath.Join(cfg.InstallRoot, componentDir(ComponentNode), versionFor(ComponentNode), "bin", "npm")
	step := PlanStep{Component: ComponentNpm, Observed: npmState}
	usable := true
	switch npmState.Compatibility {
	case CompatCompatible:
		step.Action = ActionReuse
		step.Reason = fmt.Sprintf("reuse compatible npm %s", npmState.Version)
	case CompatAbsent:
		step.Action = ActionInstall
		step.Outputs = []string{managedNpm}
		if nodeInstalling {
			step.Reason = "npm is provided by the managed Node install"
			step.Command = "(provided by the managed Node archive)"
			step.Effects = []string{
				"uses the npm shipped inside the same managed Node archive that installs Node",
			}
		} else {
			// Compatible Node exists but npm does not: install the already-pinned
			// official Node bundle into the owned root purely to provide npm. The
			// existing Node is reused for Node invocations and is never modified.
			step.Reason = "npm is missing while an existing Node was reused; install the pinned official Node bundle as an owned npm provider"
			step.Command = "download+verify+extract node (npm provider); the existing Node on PATH is left unchanged"
			step.Effects = []string{
				"downloads the pinned official Node tar.gz over https and verifies its pinned SHA-256 before extraction",
				"extracts it into the owned per-user install root solely to provide npm; the existing Node on PATH is not replaced, reconfigured, or modified",
				"no PATH, shell-profile, system-prefix, or sudo changes",
			}
		}
	default:
		step.Action = ActionBlocked
		step.Reason = blockedReason(npmState)
		usable = false
	}
	plan.Steps = append(plan.Steps, step)
	return usable
}

func planPiStep(plan *Plan, cfg Config, piState ComponentState, npmUsable bool) bool {
	switch piState.Compatibility {
	case CompatCompatible:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: ComponentPi,
			Action:    ActionReuse,
			Reason:    fmt.Sprintf("reuse compatible Pi %s", piState.Version),
			Observed:  piState,
		})
		return true
	case CompatAbsent:
		if !npmUsable {
			plan.Steps = append(plan.Steps, PlanStep{
				Component: ComponentPi,
				Action:    ActionBlocked,
				Reason:    "Pi is missing and no usable npm is planned; resolve npm first",
				Observed:  piState,
			})
			return false
		}
		prefix := filepath.Join(cfg.InstallRoot, "pi", versionFor(ComponentPi))
		plan.Steps = append(plan.Steps, PlanStep{
			Component: ComponentPi,
			Action:    ActionInstall,
			Reason:    "Pi is missing; install the pinned package into an owned npm prefix",
			Outputs:   []string{filepath.Join(prefix, "bin", "pi")},
			Effects: []string{
				fmt.Sprintf("runs `npm install --global --prefix %s --ignore-scripts %s`", prefix, PiNpmPackage),
				"scopes the child PATH, HOME, and npm cache under the owned install root; no global or system prefix, no sudo",
				"package lifecycle scripts are disabled with --ignore-scripts",
			},
			Command:  fmt.Sprintf("npm install --global --prefix %s --ignore-scripts %s", prefix, PiNpmPackage),
			Observed: piState,
		})
		return true
	default:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: ComponentPi,
			Action:    ActionBlocked,
			Reason:    blockedReason(piState),
			Observed:  piState,
		})
		return false
	}
}

func planArchiveStep(plan *Plan, cfg Config, state ComponentState, options archiveStepOptions) {
	switch state.Compatibility {
	case CompatCompatible:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: state.Component,
			Action:    ActionReuse,
			Reason:    fmt.Sprintf("reuse compatible %s %s", state.Component, state.Version),
			Observed:  state,
		})
	case CompatAbsent:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: state.Component,
			Action:    ActionInstall,
			Reason:    fmt.Sprintf("%s is missing; install the pinned candidate", options.installKind),
			Outputs:   []string{filepath.Join(cfg.InstallRoot, componentDir(state.Component), options.version)},
			Effects:   options.installEffects,
			Command:   fmt.Sprintf("download+verify+extract %s", options.installKind),
			Observed:  state,
		})
	default:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: state.Component,
			Action:    ActionBlocked,
			Reason:    blockedReason(state),
			Observed:  state,
		})
	}
}

type archiveStepOptions struct {
	binaryName     string
	version        string
	installKind    string
	installEffects []string
}

func planEnsurePackageStep(plan *Plan, cfg Config, detection Detection, component Component, packageName, source string, piUsable bool, effects []string) {
	state := stateOrAbsent(detection, component)
	step := PlanStep{Component: component, Observed: state}
	if !piUsable {
		step.Action = ActionBlocked
		step.Reason = "Pi is not usable; package registration cannot be planned"
		plan.Steps = append(plan.Steps, step)
		return
	}
	step.Action = ActionEnsure
	step.Reason = fmt.Sprintf("reuse an exact %s registration or install it after consent", packageName)
	step.Command = fmt.Sprintf("pi install %s --no-approve", source)
	step.Effects = effects
	plan.Steps = append(plan.Steps, step)
}

func planPayloadStep(plan *Plan, cfg Config, state ComponentState) {
	switch state.Compatibility {
	case CompatCompatible:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: ComponentOGSPayload,
			Action:    ActionReuse,
			Reason:    "identical embedded OGS payload is already present",
			Observed:  state,
		})
	case CompatAbsent, CompatModified:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: ComponentOGSPayload,
			Action:    ActionInstall,
			Reason:    "copy the embedded canonical OGS skills into the workspace",
			Outputs:   []string{state.Path},
			Effects: []string{
				"writes only the embedded OGS skill files under " + ogsskills.SkillsRoot + ": " + strings.Join(ogsskills.RelativePaths(), ", "),
				"never overwrites a modified or unowned existing payload file",
				"does not edit AGENTS.md, SYSTEM files, or .pi/settings.json",
			},
			Command:  "copy embedded payload",
			Observed: state,
		})
	default:
		plan.Steps = append(plan.Steps, PlanStep{
			Component: ComponentOGSPayload,
			Action:    ActionBlocked,
			Reason:    blockedReason(state),
			Observed:  state,
		})
	}
}

func stateOrAbsent(detection Detection, component Component) ComponentState {
	if state, ok := detection.State(component); ok {
		return state
	}
	return ComponentState{Component: component, Compatibility: CompatAbsent}
}

func blockedReason(state ComponentState) string {
	if state.Detail != "" {
		return "fail closed: " + state.Detail
	}
	return fmt.Sprintf("fail closed: %s could not be classified safely", state.Component)
}

func versionFor(component Component) string {
	if candidate, ok := CandidateFor(component); ok {
		return candidate.Version
	}
	return ""
}

// fingerprintOf returns an unambiguous deterministic digest of the plan's
// execution-relevant content: the normalized destination config plus every
// step's action, outputs, effects, command, and observed state. Fields are
// length-prefixed so a value containing separators cannot collide with a
// different field layout. No credential or secret is present in a Plan.
func fingerprintOf(plan Plan) string {
	hash := sha256.New()
	writeField := func(label, value string) {
		fmt.Fprintf(hash, "%d:%s=%d:%s\n", len(label), label, len(value), value)
	}
	b := plan.binding
	writeField("workspaceDir", b.WorkspaceDir)
	writeField("installRoot", b.InstallRoot)
	writeField("godotRequired", strconv.FormatBool(b.GodotRequired))
	// The public Plan.GodotRequired is a displayed field independent of the
	// normalized binding, so it is bound explicitly as well.
	writeField("planGodotRequired", strconv.FormatBool(plan.GodotRequired))
	writeField("nodePath", b.NodePath)
	writeField("npmPath", b.NpmPath)
	writeField("piPath", b.PiPath)
	writeField("engramPath", b.EngramPath)
	writeField("godotPath", b.GodotPath)
	fmt.Fprintf(hash, "steps=%d\n", len(plan.Steps))
	for _, step := range plan.Steps {
		writeField("component", string(step.Component))
		writeField("action", string(step.Action))
		writeField("reason", step.Reason)
		writeField("command", step.Command)
		fmt.Fprintf(hash, "outputs=%d\n", len(step.Outputs))
		for _, output := range step.Outputs {
			writeField("output", output)
		}
		fmt.Fprintf(hash, "effects=%d\n", len(step.Effects))
		for _, effect := range step.Effects {
			writeField("effect", effect)
		}
		writeField("observedComponent", string(step.Observed.Component))
		writeField("observedFound", strconv.FormatBool(step.Observed.Found))
		writeField("observedPath", step.Observed.Path)
		writeField("observedVersion", step.Observed.Version)
		writeField("observedCompatibility", string(step.Observed.Compatibility))
		writeField("observedDeferred", strconv.FormatBool(step.Observed.ProbeDeferred))
		writeField("observedDetail", step.Observed.Detail)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
