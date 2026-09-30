# tool-check-execution Specification

## Purpose

Define probing and result semantics for host-tool environment checks.

## Requirements

### Requirement: Independent probe execution

The system MUST execute each resolved tool check independently. A failure in one check MUST NOT prevent unrelated selected checks from running unless the caller explicitly requests fail-fast behavior.

#### Scenario: Successful probe

- GIVEN a selected `godot` check can invoke its expected binary
- WHEN the check runs
- THEN the result SHALL be success
- AND it SHALL include the detected command or path.

#### Scenario: Failed probe does not hide another result

- GIVEN `godot` is missing and `blender` is available
- WHEN both selected checks run
- THEN `godot` SHALL return failure
- AND `blender` SHALL still return success.

### Requirement: Result classification

The system SHALL classify each check result as success, warning, or failure. Failures MUST include tool id, reason, attempted command or path, and remediation guidance. Warnings SHOULD allow the flow to continue while remaining visible to users.

#### Scenario: Missing required tool fails

- GIVEN a required selected tool is unavailable
- WHEN its check runs
- THEN the result MUST be failure
- AND guidance SHALL explain how to install or configure the tool path.

#### Scenario: Non-blocking probe concern warns

- GIVEN a selected tool is found but version output is incomplete
- WHEN the check can still confirm the tool is runnable
- THEN the result SHOULD be warning
- AND the caller MAY continue.
