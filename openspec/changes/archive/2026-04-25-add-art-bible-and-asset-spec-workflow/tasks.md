# Tasks: Add Art Bible and Asset Spec Workflow

## Phase 1: Visual Workflow Foundation

- [x] 1.1 Add `VisualWorkflowVersion = "visual-workflow/v1"` and visual workflow structs/status constants to `internal/assets/types.go`.
- [x] 1.2 Add a default visual workflow constructor in `internal/assets/types.go` with art-bible/spec/manifest/audit paths and pending/non-approved statuses.
- [x] 1.3 Add prerequisite/readiness metadata to `internal/assets/catalog.go` for `comfyui-workflows` and `blender-reference-modeling` without changing `asset-manifest/v1`.

## Phase 2: Generator and Wizard Metadata

- [x] 2.1 Extend `profileTemplateData` in `internal/cli/commands/generate.go` with `VisualWorkflow` defaults from `internal/assets`.
- [x] 2.2 Extend wizard final artifact data in `internal/cli/commands/wizard.go` with `visual_workflow` defaults/readiness intent only.
- [x] 2.3 Ensure generated metadata never writes starter art/spec/audit files under `design/art/` or `design/assets/`.

## Phase 3: Profile Templates and Bindings

- [x] 3.1 Update `profile.md.tmpl` with `/art-bible`, `/asset-spec`, and `/asset-audit` guidance and explicit no-execution/no-approval boundaries.
- [x] 3.2 Update `profile.summary.md.tmpl` to summarize visual workflow readiness as metadata, not generated or approved art.
- [x] 3.3 Update `pack.config.json.tmpl` and `pattern.hybrid-map.json.tmpl` with `visual_workflow` gates separate from `asset_manifest` records.
- [x] 3.4 Add `art-bible`, `asset-spec`, and `asset-audit` workflow/profile bindings in `internal/templates/registry.go`.

## Phase 4: Tests and Verification

- [x] 4.1 Add table-driven tests in `internal/assets/types_test.go` for defaults, statuses, paths, and schema separation from `asset-manifest/v1`.
- [x] 4.2 Add tests in `internal/assets/catalog_test.go` for visual adapter prerequisites blocking readiness without approved Art Bible/spec audit.
- [x] 4.3 Extend `internal/cli/commands/generate_test.go` to assert rendered profile/config/hybrid map include workflow gates and boundary language.
- [x] 4.4 Extend `internal/cli/commands/wizard_test.go` to assert final artifacts include `visual_workflow` metadata and no generated assets/files claims.
- [x] 4.5 Run `go test ./... -count=1` and confirm no ComfyUI/Blender execution, Godot/DCC mutation, or audio/music/SFX scope appears.
