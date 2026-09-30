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

The system SHALL reuse required-check semantics from `init` and `wizard`. Flows MUST check only selected packs, tools, or profile requirements. Optional diagnostics MAY inform but MUST NOT become required/blocking.

#### Scenario: Init selected only

- GIVEN Godot is selected without Blender assets
- WHEN preflight runs
- THEN the `godot` check MUST run
- AND the `blender` check MUST NOT run.

#### Scenario: Warning visible

- GIVEN preflight returns a warning
- WHEN generation proceeds
- THEN the warning SHALL be displayed in the wizard summary.

#### Scenario: Optional diagnostic
- GIVEN an optional diagnostic reference
- WHEN required checks compose
- THEN it MUST NOT enter the blocking set.

### Requirement: Env-check unchanged

`env-check` MUST preserve explicit selection/failure semantics. Optional references MUST NOT change defaults unless explicitly requested.

#### Scenario: Default excludes optional
- GIVEN only an optional integration references a diagnostic
- WHEN `game-studio env-check` runs without selection
- THEN that diagnostic MUST NOT run.
