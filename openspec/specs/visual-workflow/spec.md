# visual-workflow Specification

## Purpose

Define metadata-only `visual-workflow/v1` contracts that guide visual production before asset adapters.

## Requirements

### Requirement: Art Bible Contract

The system MUST define an Art Bible with visual pillars, style, palette, references, constraints, negative guidance, and approval state. It MUST NOT auto-approve art/design docs.

#### Scenario: Art bible captured
- GIVEN visual direction is recorded
- WHEN `visual-workflow/v1` metadata is produced
- THEN it MUST include pillars, style, palette, references, constraints, negative guidance, and approval state.

#### Scenario: Approval withheld
- GIVEN an Art Bible has no human approval
- WHEN readiness is evaluated
- THEN approval MUST remain pending.

### Requirement: Asset Spec Contract

The system MUST define per-asset specs with asset intent, gameplay/narrative use, Art Bible links, output targets, prompt-ready metadata, import hints, and acceptance criteria.

#### Scenario: Asset spec captured
- GIVEN a visual asset is planned
- WHEN its spec is recorded
- THEN it MUST link intent, use, style guidance, targets, prompts, import hints, and acceptance criteria.

#### Scenario: Missing art bible link
- GIVEN an asset spec lacks Art Bible linkage
- WHEN readiness is checked
- THEN it MUST be marked not ready.

### Requirement: Asset Audit and Readiness

The system MUST provide a read-only audit/status proving Art Bible and Asset Spec alignment before ComfyUI/Blender adapter readiness. It MUST NOT execute ComfyUI/Blender, generate images/models, mutate engine/DCC files, or include audio/music/SFX.

#### Scenario: Ready for adapter metadata
- GIVEN an approved Art Bible and compliant Asset Spec
- WHEN audit completes
- THEN status MAY mark the asset ready for metadata-only ComfyUI/Blender adapter planning.

#### Scenario: Boundary violation
- GIVEN readiness is generated
- WHEN outputs are written
- THEN it MUST NOT claim execution, generated assets, engine mutations, or audio scope.

### Requirement: Workflow Layer Separation

The system MUST keep `visual-workflow/v1` distinct from `asset-manifest/v1`; the workflow SHALL guide enjoyable game-making through direction and review, not one-shot generation.

#### Scenario: Separate metadata layers
- GIVEN workflow and manifest data exist
- WHEN artifacts render
- THEN workflow guidance MUST NOT be serialized as asset-manifest records.
