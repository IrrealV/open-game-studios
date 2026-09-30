# Delta for installer-wizard

## MODIFIED Requirements

### Requirement: 14-step guided flow

The system MUST run a guided sequence ending with generation/validation. It MUST check selected resources, packs, or tools. Optional diagnostics MAY display but MUST NOT block.
(Previously: Fixed 14-step flow with relevant tool checks.)

#### Scenario: Completed
- GIVEN `game-studio wizard` starts
- WHEN generation is confirmed and checks pass
- THEN steps SHALL execute in order and validate.

#### Scenario: Tool failure
- GIVEN `godot` is required
- WHEN its check fails
- THEN generation MUST stop with remediation.

#### Scenario: Unselected
- GIVEN Blender is unselected
- WHEN preflight runs
- THEN `blender` MUST NOT run.

#### Scenario: Optional diagnostic
- GIVEN an optional diagnostic exists
- WHEN preflight runs
- THEN it MUST NOT block or become required.

### Requirement: Resource selection separates MCPs

The system MUST keep connectors, packs, tools, optional integrations, and MCPs distinct. Optionals MUST NOT alter `tools` or `mcp` outputs.
(Previously: Connectors/packs/tools were separate from MCPs.)

#### Scenario: Distinct
- GIVEN selections are active
- WHEN optional integrations and MCPs are chosen
- THEN neither MUST mix with tools or each other.

#### Scenario: Compatible artifact field
- GIVEN tools, MCPs, and optional integrations are selected
- WHEN artifacts generate
- THEN `tools`/`mcp` stay unchanged and optionals use a distinct field.

## ADDED Requirements

### Requirement: Optional summary

The wizard MUST show selected optionals and consent states in summaries/metadata without implying installation.

#### Scenario: Selection-only
- GIVEN `engram-monitor` is selected
- WHEN summarized
- THEN it SHALL appear optional, not installed or started.

#### Scenario: Consent surfaced
- GIVEN `metronous` is selected with consent
- WHEN artifacts generate
- THEN metadata MUST include explicit consent semantics.
