# Proposal: Add Art Bible and Asset Spec Workflow

## Intent

Add a metadata-only `visual-workflow/v1` layer before ComfyUI/Blender execution so visual production is guided by art direction, per-asset intent, and readiness checks instead of one-shot generation.

## Scope

### In Scope
- Art Bible metadata/profile guidance: visual pillars, style, palette, references, constraints, negative space/what-not-to-do, approval status.
- Asset Spec metadata: per-asset intent, gameplay/narrative use, art-bible links, targets, prompt-ready fields, import hints, acceptance criteria.
- Asset Audit/Readiness: read-only checklist/status proving specs/assets align with the art bible and are ready for adapters.
- Generated profile/pack/wizard metadata documenting `/art-bible -> /asset-spec -> adapter -> /asset-audit` gates.

### Out of Scope
- Running ComfyUI/Blender, generating images/models, mutating Godot/Blender projects, or auto-approving docs.
- Audio, music, and SFX workflows.
- New execution adapters beyond metadata for `comfyui-workflows` and `blender-reference-modeling`.

## Capabilities

### New Capabilities
- `visual-workflow`: Defines `visual-workflow/v1` art bible, asset spec, readiness, and audit metadata/gates.

### Modified Capabilities
- `asset-pipeline-contract`: Require approved art bible/spec readiness before adapter production metadata can be marked ready.
- `installer-wizard`: Surface selected visual workflow defaults/readiness metadata without implying execution.

## Approach

Model art bible/spec/audit as upstream workflow contracts, not asset adapters. Add typed/generated metadata fields and OpenCode profile guidance while preserving the current metadata-only boundary and visual-only scope.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/types.go` | Modified | Add/complement upstream visual workflow metadata. |
| `internal/assets/catalog.go` | Modified | Document adapter readiness prerequisites. |
| `internal/cli/commands/generate.go` | Modified | Pass visual workflow profile data. |
| `internal/cli/commands/wizard.go` | Modified | Record workflow defaults/readiness intent. |
| `internal/templates/assets/profile/game-studio/godot/*.tmpl` | Modified | Generate guidance/config summaries. |
| `internal/templates/registry.go` | Modified | Expose art-bible/spec/audit workflow bindings. |
| `openspec/specs/*` | Modified/New | Add `visual-workflow`, update affected specs. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Metadata confused with generated/approved art | Med | Explicit statuses and no auto-approval language. |
| Asset manifest overload | Med | Keep `visual-workflow/v1` separate from `asset-manifest/v1`. |
| CCGS skill assumptions leak in | Med | Translate to OpenCode workflow guidance only. |

## Rollback Plan

Remove `visual-workflow/v1` fields/templates/spec deltas and restore prior asset-pipeline/wizard metadata outputs.

## Dependencies

- Existing metadata-only asset adapter catalog: `comfyui-workflows`, `blender-reference-modeling`.

## Success Criteria

- [ ] Specs define art bible, asset spec, readiness, and read-only audit gates.
- [ ] Generated artifacts preserve no-execution/no-mutation/no-auto-approval claims.
- [ ] Audio/music/SFX remain explicitly out of scope.
