# tool-check-cli-integration Specification

## Purpose

Define how tool checks are exposed through CLI flows, including direct diagnostics and preflight use.

## Requirements

### Requirement: Standalone env-check command

The system MUST provide `game-studio env-check` to run environment diagnostics. The command SHALL support explicit tool selection and MUST default to the relevant configured or initial supported tool set when no selection is provided.

#### Scenario: Direct selected check

- GIVEN `game-studio env-check --tool blender`
- WHEN the command runs
- THEN only the `blender` check MUST be executed
- AND the output SHALL show its result classification.

#### Scenario: CLI failure exit

- GIVEN any selected required check fails
- WHEN `env-check` finishes
- THEN the command MUST return a failing status
- AND output SHALL identify the failed tool and remediation.

### Requirement: Init and wizard preflight composition

The system SHALL reuse the same registry and execution semantics from `init` and `wizard`. These flows MUST run only checks relevant to selected packs, tools, or profile requirements before generation.

#### Scenario: Init checks selected tools only

- GIVEN initialization selects Godot support without Blender assets
- WHEN preflight runs
- THEN the `godot` check MUST run
- AND the `blender` check MUST NOT run.

#### Scenario: Wizard warning remains visible

- GIVEN wizard preflight returns a warning for a selected tool
- WHEN generation proceeds
- THEN the warning SHALL be displayed in the wizard summary.
