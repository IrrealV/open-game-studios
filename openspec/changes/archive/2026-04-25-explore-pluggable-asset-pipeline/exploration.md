# Exploration: explore-pluggable-asset-pipeline

## Current State

`open-game-studios` is currently a Go CLI/profile generator, not an asset runtime. The concrete generation path emits Godot OpenCode profile artifacts (`profile`, `summary`, `pattern`, `pack-config`) through `internal/cli/commands/generate.go` and `internal/templates`, while `wizard` captures installer choices and writes metadata-only final artifacts. Host tools are checked through a composable `internal/toolcheck` registry, and optional integrations are intentionally selection-only metadata with no install/clone/build/run side effects.

There is no asset manifest, image generation workflow, 2D processor registry, 3D conversion adapter, validation gate, or Godot/Blender import metadata model today. Existing specs already establish the important boundary: future `asset-adapter` integrations SHOULD be metadata-only and MUST NOT trigger external actions during installer selection.

## Affected Areas

- `internal/templates/registry.go` — current extension point for engine pack metadata and skill-agent bindings; likely place to expose profile-level asset-pipeline guidance without hard-coding backend behavior.
- `internal/templates/assets/profile/game-studio/godot/*.tmpl` — generated profile/config artifacts would need asset-pipeline sections, adapter capability metadata, and import workflow instructions.
- `internal/cli/commands/wizard.go` — already captures optional integrations and final artifact metadata; should only record asset-pipeline intent/preferences, not install or run backends.
- `internal/integrations/*` — existing selection-only catalog can represent adapter/tool choices as metadata, but it is not enough for runtime asset pipeline contracts.
- `internal/toolcheck/*` — good pattern for future backend diagnostics (Blender, ComfyUI, Hunyuan3D, CUDA, Python, etc.) while keeping checks independent and selected-only.
- `openspec/specs/optional-tools-selection/spec.md` — already constrains asset-adapter selections to metadata-only behavior.
- New future package, likely `internal/assetpipeline` — should own manifests, adapter contracts, validation results, provenance, and import metadata if implementation proceeds.

## Approaches

1. **Locked single backend** — Generate one hard-coded image/3D path, for example TRELLIS.2 or Hunyuan3D-only.
   - Pros: fastest initial happy path; simpler docs and tests.
   - Cons: violates user preference; bakes heavyweight/pro hardware assumptions into core; makes fallback and platform support painful.
   - Effort: Medium.

2. **Core manifest + adapter registry** — Core models asset requests/results as manifests and invokes selected adapters behind capability contracts.
   - Pros: preserves backend/model/workflow choice; keeps image/3D conversion out of core; fits existing registry/toolcheck patterns; supports an initial catalog limited to ComfyUI workflow metadata and Blender-assisted procedural/reference modeling, while leaving ComfyUI-3D-Pack workflows and Hunyuan3D-2, TripoSR, Stable Fast 3D, and TRELLIS.2 as explicitly deferred future research rather than supported peer candidates.
   - Cons: requires careful data-model design before implementation; more specs/tests up front.
   - Effort: High.

3. **Metadata-only profile guidance first** — Extend generated profiles/configs with documented pipeline contracts and adapter selection metadata, but no runtime execution.
   - Pros: safest next step for this CLI generator; aligns with current optional-integration behavior; gives users a clear workflow without pretending conversion is magic.
   - Cons: does not generate assets yet; downstream runtime implementation remains future work.
   - Effort: Low/Medium.

## Recommendation

Proceed with approach 3 as the next proposal, while designing it so approach 2 can follow without rewrites. The near-term change should define a pluggable asset-pipeline contract in generated profile artifacts: image/prompt/provenance input, asset manifest, adapter capability declaration, validation/human approval gate, and Godot/Blender import metadata. Core must remain an orchestrator of manifests, adapters, validation, and import metadata; it must not hide image-to-3D or procedural modeling as automatic magic.

Recommended capability boundaries:

- **Core owns**: asset manifest schema, stable asset IDs, prompt/provenance metadata, requested output types, adapter selection metadata, validation status, approval state, import targets, and generated profile instructions.
- **Adapters own**: backend-specific execution, model/workflow selection, hardware requirements, output conversion, and backend diagnostics.
- **Validators own**: file existence/type checks, polygon/texture/scale metadata where available, licensing/provenance checks, and human approval status.
- **Import metadata owns**: Godot resource paths/settings and Blender scene/material hints, without mutating projects unless a future explicit command is added.

Suggested minimal manifest concepts for the spec/design phase:

- `asset_id`, `source_prompt`, `source_image`, `provenance`, `license`, `target_kind` (`sprite`, `texture`, `material`, `model3d`, `blockout`), `requested_outputs`, `adapter_id`, `adapter_capabilities`, `backend_requirements`, `outputs`, `validation`, `approval`, `import_targets`.
- Adapter contract fields: `id`, `label`, `category`, `input_kinds`, `output_formats`, `workflow_selectable`, `models_supported`, `platforms`, `hardware_profile`, `diagnostic_tool_ids`, `execution_mode` (`external`, `api-server`, `manual-assisted`, `metadata-only`).

Backend guidance for proposal:

- Initial adapter catalog is limited to **ComfyUI** workflow metadata and **Blender reference modeling**.
- Reference **ComfyUI-3D-Pack** only as a future workflow family that may run under the ComfyUI adapter after additional research; do not model it as a separate initial adapter.
- Defer **Hunyuan3D-2**, **TripoSR**, **Stable Fast 3D**, and **TRELLIS.2** to future backend research; they are rejected from the initial catalog and must not be presented as supported peer candidates.
- Keep **TRELLIS.2** specifically in future/pro-only research notes due Linux/NVIDIA/CUDA/Conda/24GB VRAM constraints.
- Add `blender-reference-modeling` only as Blender-assisted procedural/blockout modeling from a reference image, not as magical conversion.

## Risks

- Scope creep: asset generation, 3D conversion, validation, import, and installation can become multiple products if not staged.
- Hardware/platform mismatch: TRELLIS.2-class backends are not broadly installable and must never be default/required.
- False automation: generated profiles must be explicit that human approval and adapter-specific setup are required.
- Schema churn: manifests need enough versioning/extensibility to add adapters without breaking generated artifacts.
- Licensing/provenance: generated images/models need source prompt, model, workflow, and license metadata for studio use.

## Ready for Proposal

Yes — proceed to `sdd-propose` with scope limited to metadata/profile contracts and architecture boundaries for a pluggable asset pipeline. Do not implement backend execution yet; specify the manifest, adapter capability model, validation/approval gate, import metadata, and selected-only diagnostics/intent behavior.
