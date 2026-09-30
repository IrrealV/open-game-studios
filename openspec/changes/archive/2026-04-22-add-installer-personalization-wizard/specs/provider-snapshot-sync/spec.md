# provider-snapshot-sync Specification

## Purpose

Define snapshot, generation, smoke, and deferred sync/refresh behavior.

## Requirements

### Requirement: Snapshot and generation outputs

The system SHALL capture a provider/model snapshot and MUST generate Godot-first artifacts, including profile output and a final summary artifact.

#### Scenario: Generation after confirmation

- GIVEN the user confirms summary
- WHEN generation executes
- THEN profile and final artifacts SHALL be emitted
- AND generated outputs SHALL represent the selected connectors/packs/tools and MCP selections

### Requirement: Hybrid persistence write-through

The system MUST persist outcomes using hybrid behavior (OpenSpec + Engram), preserving OpenCode-only constraints.

#### Scenario: Hybrid persistence path

- GIVEN generation succeeds
- WHEN persistence runs
- THEN OpenSpec-managed config metadata SHALL be updated
- AND Engram write-through SHALL be attempted for generated metadata

### Requirement: Smoke and deferred sync/refresh placeholder

The system MUST run smoke validation before success and MUST keep sync/refresh deferred.

#### Scenario: Successful smoke completion

- GIVEN artifacts were generated
- WHEN smoke validation passes
- THEN completion SHALL indicate the runtime wizard flow completed successfully

#### Scenario: Deferred sync/refresh contract

- GIVEN OpenSpec wizard metadata is written
- WHEN inspecting sync settings
- THEN sync/refresh SHALL be marked deferred
- AND a command hint for future refresh MAY be present
