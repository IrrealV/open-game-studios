# tool-check-registry Specification

## Purpose

Define how independent host-tool checks are registered, described, and selected.

## Requirements

### Requirement: Composable tool check metadata

The system MUST register each tool check independently with stable metadata: tool id, display name, probe command expectations, severity policy, and actionable remediation text. The registry MUST include separate `godot` and `blender` checks, and MUST NOT model them as a coupled pair.

#### Scenario: Initial tools are independently registered

- GIVEN the tool-check registry is loaded
- WHEN registered checks are listed
- THEN `godot` and `blender` SHALL appear as separate entries
- AND each entry SHALL expose its own metadata.

#### Scenario: Future tool registration

- GIVEN a future tool check is added
- WHEN it registers with valid metadata
- THEN existing CLI, init, and wizard selection behavior MUST be able to include it without command rewrites.

### Requirement: Selected tool resolution

The system SHALL resolve checks from explicit tool selections and flow requirements. It MUST run only selected or relevant checks, and MUST report unknown selections as failures before probing.

#### Scenario: Only selected tool is resolved

- GIVEN `godot` and `blender` checks exist
- WHEN the caller selects only `godot`
- THEN the resolved check set MUST contain `godot`
- AND it MUST NOT contain `blender`.

#### Scenario: Unknown tool selection

- GIVEN the registry has no `maya` check
- WHEN the caller selects `maya`
- THEN resolution MUST fail with an unknown-tool result.
