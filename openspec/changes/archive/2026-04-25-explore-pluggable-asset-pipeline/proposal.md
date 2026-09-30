# Proposal: Explore Pluggable Asset Pipeline

## Intent

Establish a metadata/profile contract for a pluggable asset pipeline without executing generation, conversion, installs, clones, builds, runs, or backend API calls. Keep the initial adapter catalog intentionally small while defining what core owns versus what adapters own.

## Scope

### In Scope
- Asset manifest schema covering prompt/image provenance, outputs, validation, approval, and import targets.
- Adapter capability metadata and initial catalog entries only for ComfyUI / ComfyUI workflows and Blender reference modeling.
- Selection/profile output describing chosen asset-pipeline preferences and adapter capabilities.
- Godot and Blender import metadata boundaries that describe paths/settings/hints only.

### Out of Scope
- Real 2D/3D generation, conversion, execution, installs, clones, builds, runs, or API calls.
- Backend-specific workflow/model selection logic, hardware probing, service management, or file mutation in Godot/Blender projects.
- Initial standalone adapters for Hunyuan3D-2, TripoSR, Stable Fast 3D, TRELLIS.2, or ComfyUI-3D-Pack; these may be future ComfyUI workflow families/backends or later catalog expansion.

## Capabilities

### New Capabilities
- `asset-pipeline-contract`: Manifest schema, adapter capability metadata, validation/approval states, import metadata boundaries, and metadata-only profile output.

### Modified Capabilities
- `optional-tools-selection`: Add asset-adapter catalog metadata while preserving selection-only/no-side-effect behavior.
- `installer-wizard`: Surface asset-pipeline intent and selected adapter metadata in summaries/final artifacts without implying installation or execution.

## Approach

Use metadata-only profile guidance first. Core owns manifests, stable asset IDs, provenance/license fields, requested outputs, adapter selection metadata, validation status, human approval state, and import target metadata. Initial adapter metadata is limited to ComfyUI as the general model/workflow hub and Blender reference modeling as procedural/blockout assistance from a reference image. Adapters own actual processing/conversion, backend execution, diagnostics, and model/workflow selection.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/templates/registry.go` | Modified | Expose profile-level asset-pipeline metadata guidance. |
| `internal/templates/assets/profile/game-studio/godot/*.tmpl` | Modified | Add manifest, adapter, validation, approval, and import metadata sections. |
| `internal/cli/commands/wizard.go` | Modified | Record asset-pipeline intent/preferences only. |
| `internal/integrations/*` | Modified | Add selection-only catalog metadata for ComfyUI workflows and Blender reference modeling only. |
| `openspec/specs/*` | Modified/New | Add new contract spec and deltas for affected behavior. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Scope creep into runtime generation | High | Explicit no-execution scope and adapter/core boundary. |
| Catalog bloat before contract stabilizes | Med | Restrict initial entries to ComfyUI workflows and Blender reference modeling. |
| False automation expectations | Med | Require validation and human approval states. |
| Schema churn | Med | Include versioned/extensible manifest fields. |

## Rollback Plan

Remove generated profile metadata sections, the two initial adapter catalog entries, and related spec deltas. Since no external tools run or projects mutate, rollback is limited to repository artifacts.

## Dependencies

- Existing optional integration selection-only behavior.
- Existing tool-check registry pattern for future diagnostics references.
- Future backend research may expand ComfyUI workflow families or add standalone adapters after the metadata contract is stable.

## Success Criteria

- [ ] Specs define manifest, adapter capabilities, validation/approval states, and import metadata boundaries.
- [ ] Initial adapter catalog metadata includes only ComfyUI / ComfyUI workflows and Blender reference modeling, with no execution behavior.
- [ ] Profile/wizard output records preferences without install/clone/build/run/API side effects.
