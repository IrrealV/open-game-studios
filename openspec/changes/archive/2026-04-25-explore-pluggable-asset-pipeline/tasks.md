# Tasks: Explore Pluggable Asset Pipeline

## Phase 1: Asset Contract Foundation

- [x] 1.1 Create `internal/assets/types.go` with visual-only manifest structs, asset kind constants, provenance/license fields, requested/generated output refs, validation status, approval state, import targets, and adapter selection metadata.
- [x] 1.2 Create `internal/assets/catalog.go` with default adapter capability metadata for only `comfyui-workflows` and `blender-reference-modeling`, including workflow/model hints, Blender reference constraints, limitations, and metadata-only execution ownership.
- [x] 1.3 Add `internal/assets/types_test.go` table tests for required manifest fields, visual kind acceptance, and validation/approval separation.
- [x] 1.4 Add `internal/assets/catalog_test.go` table tests proving the initial catalog includes only the two allowed adapter IDs and rejects/defer-marks Hunyuan3D, TripoSR, Stable Fast 3D, TRELLIS.2, and ComfyUI-3D-Pack standalone entries.

## Phase 2: Integration Catalog Wiring

- [x] 2.1 Modify `internal/integrations/types.go` to reference asset-adapter capability metadata without embedding manifest/import semantics in the generic integration type.
- [x] 2.2 Modify `internal/integrations/registry.go` so the selectable registry exposes `engram-monitor`, `metronous`, `comfyui-workflows`, and `blender-reference-modeling`, while keeping asset adapters selection-only with no install/run/API behavior.
- [x] 2.3 Extend `internal/integrations/registry_test.go` to assert stable IDs, labels, categories, hints, diagnostics/consent preservation, defensive copies, and exclusion of deferred standalone 3D adapters.

## Phase 3: Wizard and Generate Metadata Output

- [x] 3.1 Modify `internal/cli/commands/wizard.go` to capture asset-pipeline intent and selected visual adapter metadata in wizard state, summaries, and `.game-studio/generated/wizard/final.artifact.json` without implying installation or execution.
- [x] 3.2 Modify `internal/cli/commands/generate.go` to add asset-pipeline metadata to `profileTemplateData`, preserving empty/default behavior when no asset preference is selected.
- [x] 3.3 Extend `internal/cli/commands/wizard_test.go` with `t.TempDir()` assertions that final artifacts include selected adapter preferences and never claim tools installed, models downloaded, APIs called, or assets generated.
- [x] 3.4 Add/extend generate command tests under `internal/cli/commands/` to assert profile template data persists ComfyUI workflow preferences and Blender reference metadata as metadata only.

## Phase 4: Profile Template Documentation

- [x] 4.1 Update `internal/templates/assets/profile/game-studio/godot/profile.md.tmpl` with manifest contract, allowed adapter catalog, validation/approval states, and explicit no-runtime-execution boundaries.
- [x] 4.2 Update `internal/templates/assets/profile/game-studio/godot/profile.summary.md.tmpl` and `pack.config.json.tmpl` to surface selected asset-pipeline preferences, import metadata hints, and future-scope exclusions.
- [x] 4.3 Add content assertions for rendered profile/summary/pack-config output covering import metadata only and no Godot/Blender file mutation claims.

## Phase 5: Verification

- [x] 5.1 Run `go test ./... -count=1` and fix only implementation/test issues related to this change.
- [x] 5.2 Review generated artifacts/spec scenarios to confirm the contract remains visual-only, only ComfyUI/Blender adapters appear, and no task introduced runtime backend execution.
