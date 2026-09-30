# optional-tools-selection Specification

## Purpose

Selection-only integrations, separate from tools, MCPs, and checks.

## Requirements

### Requirement: Catalog

The system MUST expose stable-ID integrations with label, category, default, hints, diagnostics, and consent metadata. Initial IDs MUST include `engram-monitor` and `metronous`. Future asset-adapter categories SHOULD be metadata-only.

#### Scenario: Initial
- GIVEN the catalog is listed
- WHEN entries render
- THEN both initial IDs MUST appear outside tools/MCPs.

#### Scenario: Adapter metadata
- GIVEN an asset-adapter entry
- WHEN selected
- THEN no install, clone, build, or run SHALL occur.

### Requirement: Intent only

Selections MUST record intent only and MUST NOT install, clone, configure, start, stop, or mutate services, MCPs, shells, or OS config.

#### Scenario: No side effect
- GIVEN `engram-monitor` is selected
- WHEN accepted
- THEN intent MUST persist without external action.

### Requirement: Metronous consent

`metronous` MUST require explicit telemetry/privacy consent and distinguish selected-with-consent from absent/declined/missing.

#### Scenario: Granted
- GIVEN explicit Metronous consent
- WHEN selected
- THEN selection and affirmative consent MUST persist.

#### Scenario: Missing
- GIVEN no explicit Metronous consent
- WHEN submitted
- THEN `metronous` MUST NOT be selected.
