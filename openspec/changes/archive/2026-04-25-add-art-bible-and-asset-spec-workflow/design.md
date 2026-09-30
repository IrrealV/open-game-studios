# Design: Add Art Bible and Asset Spec Workflow

## Technical Approach

Add a new metadata-only `visual-workflow/v1` contract beside the existing `asset-manifest/v1` adapter/import contract. The workflow layer describes art-bible, asset-spec, readiness, and read-only audit gates, then exposes those fields through generated OpenCode profile docs, pack config, hybrid map metadata, and wizard final artifacts. It does not execute ComfyUI/Blender, author approved creative content, mutate Godot/DCC files, or cover audio/music/SFX.

## Architecture Decisions

| Decision | Choice | Alternatives considered | Rationale |
|---|---|---|---|
| Separate schema | Create `visual-workflow/v1` structs/defaults in `internal/assets` while retaining `asset-manifest/v1` unchanged | Merge art-bible fields into `AssetManifest` | Existing manifest is asset/adapter/import oriented; upstream craft gates need separate status and provenance without overloading backend metadata. |
| Profile data seam | Add `VisualWorkflow` to `profileTemplateData` and wizard artifact payloads | Hardcode prose only in templates | Current generator already centralizes asset metadata through `defaultAssetPipelineProfileData`; adding a parallel default keeps templates deterministic and testable. |
| Guidance, not mechanics | Generate OpenCode profile/workdoc guidance for `/art-bible`, `/asset-spec`, `/asset-audit` behavior | Copy CCGS Claude Task/AskUserQuestion orchestration | OpenCode profiles should express workflow contracts and review gates, not Claude-specific agent mechanics. |
| No starter doc mutation | Record default paths/templates/readiness in metadata; do not write `design/art/*` or `design/assets/*` files | Emit empty art bible/spec/audit markdown files | Placeholder creative docs can be mistaken for approved direction; this product must support iteration/coherence, not one-shot generation. |

## Data Flow

```text
wizard/generate defaults
  ├─ visual-workflow/v1 metadata ──→ profileTemplateData.VisualWorkflow
  │                                   ├─ profile.md guidance
  │                                   ├─ summary.md readiness overview
  │                                   ├─ pack.config.json machine contract
  │                                   └─ hybrid-map.json workflow gates
  └─ asset-manifest/v1 metadata ───→ existing AssetPipeline adapter/import contract

Art Bible approved → Asset Spec ready → Adapter metadata eligible → Asset Audit read-only → Approval/import metadata
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/types.go` | Modify | Add `VisualWorkflowVersion`, art-bible/spec/readiness/audit structs, statuses, paths, and default constructor. Keep `SchemaVersion` for asset manifests unchanged. |
| `internal/assets/catalog.go` | Modify | Add adapter prerequisite metadata stating visual adapters require approved art-bible/spec readiness before production metadata is considered ready. |
| `internal/cli/commands/generate.go` | Modify | Extend `profileTemplateData` with `visualWorkflowProfileData` and default values. |
| `internal/cli/commands/wizard.go` | Modify | Add wizard visual workflow intent/defaults/readiness payload without changing optional adapter selection semantics. |
| `internal/templates/assets/profile/game-studio/godot/*.tmpl` | Modify | Render Art Bible, Asset Spec, Asset Audit/Readiness guidance, paths, gates, and no-execution boundary in profile, summary, config, and hybrid map. |
| `internal/templates/registry.go` | Modify | Add skill-agent bindings for `art-bible`, `asset-spec`, and `asset-audit` as workflow/profile guidance roles. |
| `internal/assets/*_test.go`, `internal/cli/commands/*_test.go`, `internal/templates/*_test.go` | Modify | Add table-driven/unit assertions for schemas, defaults, payloads, rendered artifacts, and no-execution claims. |

## Interfaces / Contracts

```go
const VisualWorkflowVersion = "visual-workflow/v1"

type VisualWorkflowContract struct {
    Version string `json:"version"`
    Boundary string `json:"boundary"`
    ArtBible VisualArtBibleContract `json:"art_bible"`
    AssetSpec VisualAssetSpecContract `json:"asset_spec"`
    Readiness VisualReadinessContract `json:"asset_readiness"`
    Audit VisualAuditContract `json:"asset_audit"`
    Gates []WorkflowGate `json:"workflow_gates"`
}
```

Default paths: `design/art/art-bible.md`, `design/assets/specs/`, `design/assets/asset-manifest.md`, `design/assets/audits/latest.md`. Statuses should include `missing`, `draft`, `approved`, `concerns`, `ready_for_adapter`, `audited`; audit remains read-only.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Visual workflow defaults, status values, manifest separation | Table-driven Go tests in `internal/assets`; assert schema versions differ. |
| Integration | Generated profile/config/hybrid map include workflow gates and no-execution claims | Use `t.TempDir()` around existing generate helpers; parse JSON where applicable. |
| Wizard | Final artifact includes visual workflow defaults but no generated assets/files | Existing wizard tests extended with non-interactive and selected-adapter cases. |

## Migration / Rollout

No migration required. Existing `asset-manifest/v1` JSON remains compatible; generated artifacts gain additive `visual_workflow` metadata.

## Open Questions

- [ ] None blocking.
