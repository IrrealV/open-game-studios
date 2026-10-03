package cli

import (
	"errors"
	"fmt"

	"open-game-studios/internal/cli/commands"
	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
)

type App struct {
	templates   *templates.Registry
	persistence persistence.Placeholder
}

func New() *App {
	return &App{
		templates:   templates.NewRegistry(),
		persistence: persistence.NewPlaceholder(),
	}
}

func (a *App) Run(args []string) error {
	if len(args) == 0 {
		return errors.New(a.help())
	}

	switch args[0] {
	case "init":
		return commands.RunInit(commands.InitInput{
			Args:        args[1:],
			Persistence: a.persistence,
			EnvChecker:  commands.DefaultEnvironmentChecker(),
		})
	case "env-check":
		return commands.RunEnvCheck(commands.EnvCheckInput{
			Args: args[1:],
		})
	case "generate":
		return commands.RunGenerate(commands.GenerateInput{
			Args:        args[1:],
			Registry:    a.templates,
			Persistence: a.persistence,
		})
	case "smoke":
		return commands.RunSmoke(commands.SmokeInput{
			Args:     args[1:],
			Registry: a.templates,
		})
	case "smoke-suite":
		return commands.RunSmokeSuite(commands.SmokeSuiteInput{
			Args:     args[1:],
			Registry: a.templates,
		})
	case "wizard":
		return commands.RunWizard(commands.WizardInput{
			Args:        args[1:],
			Registry:    a.templates,
			Persistence: a.persistence,
			EnvChecker:  commands.DefaultEnvironmentChecker(),
			Installer:   commands.DefaultWizardInstaller(),
		})
	case "brief":
		return commands.RunBrief(commands.BriefInput{
			Args: args[1:],
		})
	case "visual":
		return commands.RunVisual(commands.VisualInput{
			Args: args[1:],
		})
	case "help", "-h", "--help":
		fmt.Print(a.help())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], a.help())
	}
}

func (a *App) help() string {
	return `game-studio: Pi-first Game-Studio installer/generator

Usage:
  game-studio <command> [options]

Commands:
  init        Initialize and verify Game-Studio workspace structure
  env-check   Run host tool diagnostics (Godot, Blender, or selected tools)
  generate    Generate studio profile + pack artifacts (flat studio-profile.<engine>.md names)
  wizard      Run staged installer/personalization wizard (Pi only)
  smoke       Validate generated artifact layout and metadata
  smoke-suite Run deterministic smoke harness + report
  brief       Build a pending core-game draft, or check one against a recorded decision
  visual      Validate Art Bible / Asset Spec docs and run a read-only visual audit

Notes:
  - The staged wizard targets Pi only. It prepares a fingerprint-bound prerequisite plan and, after explicit approval, installs missing components; --metadata-only keeps artifact-only generation.
  - Generate flat layout emits studio-profile.<engine>.md / studio-profile.<engine>.summary.md; pack layout keeps profile.md / summary.md.
  - MCP adapters are optional and never required; memory uses the Pi-native Engram companion.
  - Godot generation path is the only supported concrete engine output today.
  - CCGS semantics are preserved through layered registry metadata.
  - brief draft writes only pending artifacts; brief check is consistency validation of a claimed recorded decision and grants no authority.
  - visual is metadata only: it never executes ComfyUI/Blender, generates assets, mutates Godot/DCC files, or self-approves an Art Bible.
`
}
