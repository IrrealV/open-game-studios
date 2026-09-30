# asset-pipeline-contract Specification

## Purpose

Define metadata-only contracts for visual asset generation workflows covering 2D images/assets and 3D blockouts/models/import metadata.

## Requirements

### Requirement: Visual Asset Manifest

The system MUST define a versioned visual asset manifest with stable asset ID, visual asset kind, prompt/reference provenance, license/source notes, requested outputs, generated output references, validation state, approval state, and import targets.

#### Scenario: Manifest captured
- GIVEN a visual asset request
- WHEN manifest metadata is recorded
- THEN it MUST include provenance, requested outputs, validation, approval, and import targets.

### Requirement: Adapter Capability Metadata

The system MUST describe adapters by stable ID, label, category, supported visual asset kinds, input types, output metadata types, execution ownership, and limitations. Initial adapter IDs SHALL be limited to `comfyui-workflows` and `blender-reference-modeling`.

#### Scenario: Initial catalog
- GIVEN adapter capabilities are listed
- WHEN the initial catalog renders
- THEN only ComfyUI workflows and Blender reference modeling MUST appear.

#### Scenario: Unsupported adapter
- GIVEN Hunyuan3D, TripoSR, Stable Fast 3D, or TRELLIS.2 is proposed
- WHEN validating the initial catalog
- THEN it MUST be rejected as future scope.

### Requirement: Backend and Workflow Selection Metadata

The system MUST record selected backend/model/workflow preferences as metadata only. It MUST NOT install, clone, build, run, probe hardware, call APIs, manage services, or execute generation.

#### Scenario: Preference saved
- GIVEN a user selects ComfyUI workflow preferences
- WHEN profile metadata is generated
- THEN preferences MUST persist without backend execution.

### Requirement: Validation, Approval, and Import Boundaries

The system MUST distinguish visual-workflow readiness, manifest validation status, and human approval state. Adapter production metadata SHALL NOT be marked ready unless the related `visual-workflow/v1` Art Bible is human-approved and the Asset Spec passes read-only audit/readiness. Godot and Blender import metadata SHALL describe target paths, settings, hints, and ownership boundaries only; adapters SHALL own processing/conversion in future changes.

#### Scenario: Awaiting approval
- GIVEN outputs pass metadata validation
- WHEN human review is pending
- THEN approval state MUST remain separate from validation status.

#### Scenario: Import metadata only
- GIVEN Godot or Blender import targets are declared
- WHEN metadata is generated
- THEN no project files SHALL be mutated.

#### Scenario: Visual workflow gate blocks adapter readiness
- GIVEN an asset lacks approved Art Bible or passing Asset Spec audit
- WHEN adapter production metadata is evaluated
- THEN it MUST NOT be marked ready.
