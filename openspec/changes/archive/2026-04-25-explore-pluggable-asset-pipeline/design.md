# Design: Explore Pluggable Asset Pipeline

## Technical Approach

Extend the existing Go profile-generator metadata path, not runtime behavior. The contract stays visual-only: 2D sprites/textures/concept/reference images and 3D blockouts/models/import metadata. Core will expose stable manifest/profile fields; adapters remain catalog metadata with no installs, service orchestration, API calls, conversion, or execution.

## Architecture Decisions

| Decision | Choice | Alternatives considered | Rationale |
|---|---|---|---|
| Metadata boundary | Keep asset contracts in profile/template data and integration catalog metadata. | Add runtime pipeline services. | Existing `generate`/`wizard` flows already emit metadata artifacts; runtime execution is explicitly out of scope. |
| Adapter catalog | Initial entries only: `comfyui-workflows` and `blender-reference-modeling`. | Add Hunyuan3D, TripoSR, StableFast3D, TRELLIS, ComfyUI-3D-Pack. | Prevent catalog bloat while preserving workflow/model-choice metadata for ComfyUI and procedural constraints for Blender. |
| Package boundary | Add `internal/assets` for manifest/catalog contracts; keep `internal/integrations` selection-facing. | Put all fields in `integrations.Integration`. | Avoid overloading optional integrations with asset-specific validation/import semantics. |
| Approval model | Versioned states: `draft`, `validated`, `needs_revision`, `approved`, `imported`, `rejected`. | Boolean approved flag. | Human review and import readiness need auditable intermediate states. |

## Data Flow

```text
wizard/generate intent ──→ internal/assets contracts
        │                         │
        ├──→ internal/integrations catalog metadata
        │                         │
        └──→ template data ──→ profile/summary/pack-config artifacts
                                  │
                                  └── manifest examples/import boundaries only
```

No generated artifact may imply files were created in Godot/Blender projects or that ComfyUI/Blender executed.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/types.go` | Create | Visual asset manifest, provenance, validation/approval, import target, adapter capability structs. |
| `internal/assets/catalog.go` | Create | Metadata-only default adapter catalog for ComfyUI workflows and Blender reference modeling. |
| `internal/integrations/types.go` | Modify | Reference asset adapter selections without embedding manifest semantics. |
| `internal/integrations/registry.go` | Modify | Surface selectable metadata/hints for the two initial visual asset adapters only. |
| `internal/cli/commands/wizard.go` | Modify | Record asset-pipeline preferences in state/final artifact/profile markdown. |
| `internal/cli/commands/generate.go` | Modify | Add asset pipeline metadata to `profileTemplateData`. |
| `internal/templates/assets/profile/game-studio/godot/*.tmpl` | Modify | Document manifest contract, catalog entries, approval states, Godot/Blender import metadata boundaries. |

## Interfaces / Contracts

```go
type AssetManifest struct {
    SchemaVersion string
    AssetID string
    Kind string // sprite|texture|concept|reference-image|blockout|model|import-metadata
    Provenance AssetProvenance
    RequestedOutputs []RequestedOutput
    AdapterSelection AdapterSelection
    Validation ValidationState
    Approval ApprovalState
    ImportTargets []ImportTarget // godot|blender metadata only
}
```

ComfyUI metadata preserves `workflow_family`, `workflow_name`, `model_hint`, `checkpoint_hint`, `loras`, and required inputs. Blender reference modeling metadata preserves `reference_image`, `scale_units`, `style_constraints`, `poly_budget_hint`, `allowed_operations`, and export/import hints.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | `internal/assets` validation/default catalog. | Table-driven Go tests. |
| Unit | Integration registry IDs, hints, defensive copies. | Extend existing registry tests. |
| Integration | Wizard/generate metadata output. | `t.TempDir()` and content assertions; no external tools. |
| E2E | None. | Not needed; no runtime pipeline exists. |

## Migration / Rollout

No data migration required. Roll out behind metadata-only defaults: non-interactive wizard may emit no asset preference unless explicitly selected.

## Open Questions

- [ ] Exact CLI prompt UX for asset preferences can be finalized in tasks/spec, but does not block this design.
