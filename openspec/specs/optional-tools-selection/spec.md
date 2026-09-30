# optional-tools-selection Specification

## Purpose

Selection-only integrations, separate from tools, MCPs, and checks.

## Requirements

### Requirement: Catalog

The system MUST expose stable-ID integrations with label, category, default, hints, diagnostics, consent metadata, and metadata-only asset-adapter capability descriptors. Initial IDs MUST include `engram-monitor`, `metronous`, `comfyui-workflows`, and `blender-reference-modeling`; initial asset-adapter IDs MUST NOT include Hunyuan3D, TripoSR, Stable Fast 3D, TRELLIS.2, or ComfyUI-3D-Pack standalone entries.
(Previously: Catalog exposed initial optional integrations and allowed future asset-adapter categories without naming the initial asset adapter catalog.)

#### Scenario: Initial
- GIVEN the catalog is listed
- WHEN entries render
- THEN both initial non-asset IDs MUST appear outside tools/MCPs.

#### Scenario: Adapter metadata
- GIVEN an asset-adapter entry
- WHEN selected
- THEN no install, clone, build, run, API call, or backend execution SHALL occur.

#### Scenario: Initial visual adapter catalog
- GIVEN asset-adapter catalog metadata is listed
- WHEN entries render
- THEN only `comfyui-workflows` and `blender-reference-modeling` MUST appear.

#### Scenario: Deferred adapter excluded
- GIVEN a deferred 3D adapter is requested as an initial standalone entry
- WHEN the catalog validates
- THEN the entry MUST be excluded before generic unknown-ID handling with future-scope metadata.

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
