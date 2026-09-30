# Delta for tool-check-cli-integration

## MODIFIED Requirements

### Requirement: Init and wizard preflight composition

The system SHALL reuse required-check semantics from `init` and `wizard`. Flows MUST check only selected packs, tools, or profile requirements. Optional diagnostics MAY inform but MUST NOT become required/blocking.
(Previously: Preflight checked selected packs, tools, or profile requirements.)

#### Scenario: Init selected only
- GIVEN Godot is selected without Blender assets
- WHEN preflight runs
- THEN `godot` MUST run and `blender` MUST NOT run.

#### Scenario: Warning visible
- GIVEN preflight returns a warning
- WHEN generation proceeds
- THEN the warning SHALL appear in the summary.

#### Scenario: Optional diagnostic
- GIVEN an optional diagnostic reference
- WHEN required checks compose
- THEN it MUST NOT enter the blocking set.

## ADDED Requirements

### Requirement: Env-check unchanged

`env-check` MUST preserve explicit selection/failure semantics. Optional references MUST NOT change defaults unless explicitly requested.

#### Scenario: Default excludes optional
- GIVEN only an optional integration references a diagnostic
- WHEN `game-studio env-check` runs without selection
- THEN that diagnostic MUST NOT run.
