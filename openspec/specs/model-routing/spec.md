# Spec: Model Routing

## Purpose

Define the `model-routing/v1` metadata contract for routing Game-Studio phases to production capabilities without executing providers, downloading models, generating assets, mutating engines, or approving artifacts.

## Boundary

The contract is declarative metadata only. Pi owns authentication and actual
model selection; OGS does not inspect providers or inventory models. The contract
MUST NOT perform provider API calls, model downloads, image generation, image
review execution, audio generation, audio review execution, SDD phase execution,
Godot mutation, ComfyUI execution, Blender execution, or auto-approval.

## Requirement: Capability catalog

`model-routing/v1` MUST define core capabilities `default_text`, `reasoning_heavy`, `fast_text`, `code_apply`, and `qa_review`; optional non-future capabilities `archive_summary`, `visual_review`, and `godot_technical_review`; and future/deferred media capabilities `image_generation`, `image_review`, `audio_generation`, and `audio_review`.

### Scenario: Capabilities are machine-readable

- GIVEN a generated profile, hybrid-map, pack config, or wizard final artifact
- WHEN model routing metadata is inspected
- THEN it MUST include `model-routing/v1`
- AND it MUST include capability entries with id, label, category, requirement, selection state, configured status, provider, model, fallback if applicable, validation status, notes, and boundary.

## Requirement: Phase routing from core-game-workflow/v1

Routing MUST reuse phase IDs from `core-game-workflow/v1`: `game-concept`, `game-pillars`, `core-loop`, `player-fantasy`, `mechanics-brief`, `narrative-brief`, `tone-and-mood`, `story-constraints`, `gdd-slice`, `change-brief`, and `repair-brief`.

### Scenario: Recommended routing maps production phases to capabilities

- GIVEN the recommended setup preset
- WHEN phase routes are inspected
- THEN game design, narrative, GDD, change, and repair phases MUST route to `reasoning_heavy`
- AND Godot handoff or SDD apply metadata MUST route to `code_apply`
- AND archive or summary metadata MUST route to `fast_text` or `archive_summary`
- AND QA/review gates MUST route to `qa_review`.

## Requirement: Provider/model bindings are not hardcoded

Provider/model bindings MUST default to `not_configured` with the canonical
`binding_source: manual_or_runtime_owned` unless the user explicitly configures a
provider/model binding. Bindings are manual user preferences: the Pi runtime owns
actual model selection and authentication, and OGS does not inspect providers or
inventory models. The deprecated legacy `binding_source:
manual_or_future_opencode_discovery` MUST remain accepted for historical
serialized inputs, and any other value MUST be rejected.

### Scenario: Clean platform release gate does not assume providers

- GIVEN a clean Windows, macOS, or Linux machine
- WHEN generated metadata is inspected
- THEN no provider or model name MUST be assumed
- AND bindings requiring setup MUST set `requires_user_configuration: true`
- AND `binding_source` MUST be `manual_or_runtime_owned`.

### Scenario: Deprecated legacy binding source is accepted for historical inputs

- GIVEN a historical `model-routing/v1` input with `binding_source: manual_or_future_opencode_discovery`
- WHEN the contract is validated
- THEN it MUST remain accepted
- AND the value MUST be treated as deprecated, not as an alias of the canonical source.

### Scenario: Unknown binding source is rejected

- GIVEN a `model-routing/v1` input with an unrecognized `binding_source`
- WHEN the contract is validated
- THEN validation MUST reject it.

## Requirement: Preset compatibility

Model routing MUST preserve the staged wizard and setup taxonomy presets `minimal`, `recommended`, `full`, and `custom`.

### Scenario: Minimal does not block on future media lanes

- GIVEN minimal setup
- WHEN image/audio capabilities are `not_configured` or not selected
- THEN validation MUST remain non-blocking.

### Scenario: Full does not imply future readiness

- GIVEN full setup
- WHEN visual/audio future capabilities are present but `not_configured`
- THEN they MUST NOT be represented as ready
- AND they MUST NOT block the final artifact.

### Scenario: Custom overrides are metadata only

- GIVEN custom setup
- WHEN overrides are inspected
- THEN `capability_overrides`, `phase_overrides`, `override_status`, and `validation_result` MUST be available as metadata
- AND no provider execution MUST occur.

## Requirement: Validation severity integration

Routing validation MUST use existing scoped severities: `pass`, `warning`, `blocker`, `skipped`, `deferred`, and `not_selected`.

### Scenario: Required and future lanes produce appropriate severity

- GIVEN selected required capabilities without usable bindings
- WHEN routing validation runs
- THEN they MAY produce warnings or blockers according to the current flow
- AND optional/future capabilities without bindings MUST be `warning`, `deferred`, or `not_selected`, not blockers unless explicitly selected as required by a future workflow.

## Relationship to existing specs

- #17 / `core-game-workflow/v1`: source of core phase IDs and human approval boundary.
- #28 staged wizard: consumes routing metadata without redesigning the wizard.
- #31 presets/taxonomy: preserves minimal/recommended/full/custom behavior.
- #29 scoped validation: uses scoped severities and non-blocking deferred lanes.
- #32 visual source lanes and #30 provider-native image generation remain out of scope.
