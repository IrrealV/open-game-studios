# Delta for installer-wizard

## MODIFIED Requirements

### Requirement: 14-step guided flow

The system MUST run a 14-step sequence (0..14), ending with generation and smoke validation. Before generation, the wizard MUST run environment preflight checks only for tools relevant to the user's selected profile resources, packs, or tools. Blocking failures MUST stop generation with actionable guidance.
(Previously: The flow executed generation and smoke validation without explicit selected-tool environment preflight checks.)

#### Scenario: Full completed flow

- GIVEN `game-studio wizard` starts
- WHEN the user confirms generation and relevant tool checks pass
- THEN steps 0..14 SHALL execute in order
- AND step 14 SHALL complete runtime smoke validation before reporting success.

#### Scenario: Selected tool preflight failure blocks generation

- GIVEN the wizard selection requires `godot`
- WHEN the `godot` check fails before generation
- THEN generation MUST NOT start
- AND the wizard SHALL report the failed tool, reason, attempted command or path, and remediation guidance.

#### Scenario: Unselected tool is not checked

- GIVEN the wizard selection does not require Blender
- WHEN preflight runs
- THEN the `blender` check MUST NOT run.
