# provider-snapshot-sync Specification

## Purpose

Define generated artifacts, hybrid persistence, smoke validation, and deferred
sync/refresh behavior for the Pi-only wizard. There is no provider snapshot
capture: Pi owns authentication and models and OGS does not probe providers,
models, or accounts.

> Superseded: the earlier provider/model snapshot capture and the OpenCode-only
> persistence constraint are retired by the authorized Pi-only direction.
> Archived OpenSpec changes are not rewritten.

## Requirements

### Requirement: Generation outputs without provider probing

The system SHALL generate Godot-first artifacts, including the profile output and
a final summary artifact, from local selections. It MUST NOT capture a provider
or model snapshot and MUST NOT probe Pi or any provider account.

#### Scenario: Generation after confirmation

- GIVEN the user confirms generation, or the run proceeds metadata-only
- WHEN generation executes
- THEN profile and final artifacts SHALL be emitted
- AND generated outputs SHALL represent the selected connectors/packs/tools and optional integration selections
- AND provider/model ownership SHALL be recorded as not-inspected.

### Requirement: Hybrid persistence write-through

The system MUST persist outcomes using hybrid behavior (OpenSpec + Engram)
through the Pi-native Engram companion. A missing or failed Engram write-through
MUST be reported as an explicit non-success memory outcome rather than a silent
success.

#### Scenario: Hybrid persistence path

- GIVEN generation succeeds
- WHEN persistence runs
- THEN OpenSpec-managed config metadata SHALL be updated
- AND Engram write-through SHALL be attempted for generated metadata
- AND the memory outcome SHALL be recorded as pending, saved, failed, canceled, or disabled.

### Requirement: Smoke and deferred sync/refresh placeholder

The system MUST run smoke validation before success and MUST keep sync/refresh
deferred.

#### Scenario: Successful smoke completion

- GIVEN artifacts were generated
- WHEN smoke validation passes
- THEN completion SHALL indicate the staged wizard flow completed.

#### Scenario: Deferred sync/refresh contract

- GIVEN OpenSpec wizard metadata is written
- WHEN inspecting sync settings
- THEN sync/refresh SHALL be marked deferred
- AND a command hint for future refresh MAY be present.
