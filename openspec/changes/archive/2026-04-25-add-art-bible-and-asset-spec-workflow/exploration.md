# Exploration: add-art-bible-and-asset-spec-workflow

## Current State

`open-game-studios` is a Go CLI/profile generator for an OpenCode Game-Studio profile. It currently emits Godot profile artifacts, pack config, hybrid-map JSON, wizard profile/final artifact metadata, and OpenSpec state. The visual asset pipeline is intentionally metadata-only: `internal/assets` defines `asset-manifest/v1`, adapter capability metadata, validation/approval/import fields, and a default adapter catalog limited to `comfyui-workflows` and `blender-reference-modeling`.

The existing pipeline starts too late for disciplined game production. It can describe visual asset manifests and selected adapters, but it does not yet model the upstream creative workflow that should gate production: art bible creation, per-asset spec generation from approved design docs, and read-only asset readiness/audit metadata.

Relevant CCGS source skills establish this order:
- `/art-bible` runs after brainstorm/game concept and before GDD/asset production; it writes `design/art/art-bible.md` and defines visual identity, mood, shape language, color system, production guides, asset standards, and reference direction.
- `/asset-spec` runs after art bible and GDD/level/character docs; it writes `design/assets/specs/[target]-assets.md`, updates `design/assets/asset-manifest.md`, assigns stable `ASSET-NNN` IDs, and generates visual descriptions plus AI image prompts anchored to the art bible.
- `/asset-audit` is read-only; it scans assets for naming, format, budget, missing, and orphaned asset issues and produces an audit report.

## Affected Areas

- `internal/assets/types.go` — existing manifest should be extended or complemented with upstream fields: art-bible gate state, asset-spec source links, readiness state, audit findings, stable ASSET IDs, standards provenance, and prompt metadata.
- `internal/assets/catalog.go` — adapter catalog should remain execution-free, but should declare that adapters require approved art bible/spec metadata before production.
- `internal/cli/commands/generate.go` — `assetPipelineProfileData` is the profile-template data seam for generated workflow metadata and can carry required workdoc names/paths/gates.
- `internal/cli/commands/wizard.go` — optional integration/wizard final artifact can surface art bible/spec/audit readiness intent and selected visual workflow metadata without executing tools.
- `internal/templates/assets/profile/game-studio/godot/*.tmpl` — generated OpenCode profile, summary, pack config, and possibly hybrid map should document the workflow sequence and artifact contracts.
- `internal/templates/registry.go` — skill-agent mapping currently lacks art-bible/asset-spec/asset-audit workflow bindings; generated profiles should expose these as workflow skills/commands rather than runtime asset backends.
- `openspec/specs/asset-pipeline-contract/spec.md` — should gain or be paired with requirements for upstream visual workflow contracts.
- `openspec/specs/installer-wizard/spec.md` — wizard summary/final artifact requirements should include workflow readiness metadata while preserving no-execution claims.

## Data Contracts and Generated Artifacts

Recommended contract additions:
- `visual_workflow.version`: e.g. `visual-workflow/v1`, separate from `asset-manifest/v1` to avoid overloading backend adapter metadata.
- `art_bible`: path (`design/art/art-bible.md`), status (`missing|draft|approved|concerns|revised`), required sections, sign-off mode, visual identity anchor, asset standards summary reference.
- `asset_spec`: manifest path (`design/assets/asset-manifest.md`), specs directory (`design/assets/specs/`), target types (`system|level|character`), stable ID pattern (`ASSET-NNN`), prompt fields, art bible anchor citations, technical standards citations.
- `asset_readiness`: states such as `concept_only`, `art_bible_required`, `spec_required`, `ready_for_adapter`, `generated_external`, `validated`, `approved`, `import_metadata_ready`, `audited`.
- `asset_audit`: read-only report path, scanned roots, checks (`naming|format|budget|missing|orphaned|standards`), verdict (`compliant|warnings|non_compliant`), findings summary.
- `workflow_gates`: art bible MUST precede asset specs; asset specs MUST precede adapter production; audit MUST NOT mutate assets.

Generated artifacts should remain workdocs/profile metadata for this change:
- Profile docs: explain `/art-bible -> /asset-spec -> visual adapter -> /asset-audit -> approval/import metadata`.
- Pack config JSON: expose machine-readable paths, gate statuses, readiness states, and no-execution boundary.
- Wizard final artifact: record user intent and selected workflow defaults, but not create art/design files unless a future explicit command does so.
- Optional future generated empty templates may be useful, but should be treated carefully; this change should not auto-author art bible or asset specs.

## Approaches

1. **Metadata profile sections only** — Add workflow documentation and machine-readable config fields to generated artifacts.
   - Pros: lowest risk; fits current generator; preserves no-execution boundary.
   - Cons: does not provide invocable OpenCode workflow skill instructions by itself.
   - Effort: Low.

2. **Generated workdocs only** — Emit starter art bible/spec/audit markdown templates into project design paths.
   - Pros: visible artifacts for users; closer to CCGS project layout.
   - Cons: risks implying content is authored/approved; creates design files without collaborative gates.
   - Effort: Medium.

3. **Workflow skills + metadata contracts** — Add generated profile guidance/pack config plus skill-agent bindings or bundled workflow docs for art-bible, asset-spec, and asset-audit behavior.
   - Pros: best match to product vision; makes design discipline the upstream workflow layer before ComfyUI/Blender execution; keeps current code metadata-only.
   - Cons: requires careful scoping so copied CCGS skill behavior does not rely on Claude-only tools/sub-agents or audio scope.
   - Effort: Medium.

## Recommendation

Proceed with approach 3, implemented in stages. The next proposal should define a `visual-workflow/v1` metadata/profile contract and generated OpenCode workflow guidance for art bible, asset spec, and asset audit. Do not implement actual ComfyUI/Blender execution and do not auto-generate final creative content.

The change should treat art bible/spec/audit as upstream workflow layer plus metadata contracts, not as asset adapters. Use existing `internal/assets` for shared visual vocabulary where useful, but keep art-direction workflow state distinct from backend adapter capability metadata. Audio/music/SFX should remain excluded or described only as out of scope until visual workflows are complete.

## Risks

- CCGS skills use Claude-specific Task/AskUserQuestion assumptions; OpenCode profile generation should translate behavior into workflow instructions/contracts, not blindly copy execution mechanics.
- Existing `asset-manifest/v1` is backend/import oriented; overloading it with art bible/spec/audit concepts could create a confusing schema.
- Auto-writing placeholder design docs can create false confidence that art direction was approved. Prefer metadata and guidance first.
- Asset-spec source includes audio categories; this project should explicitly scope audio/music/SFX out until visual workflows are finished.
- Wizard non-interactive mode currently selects no optional integrations; defaults must not imply readiness for asset production without an approved art bible and specs.

## Ready for Proposal

Yes. Run `sdd-propose` next for `add-art-bible-and-asset-spec-workflow` with scope limited to metadata/profile/workflow contracts and generated guidance. The proposal should include rollback by removing the new visual workflow fields/templates/spec deltas, and should explicitly preserve the no-execution boundary.
