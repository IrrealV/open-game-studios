# Proposal: Add Environment Tool Checks

## Intent

Add a reusable environment-check framework so `game-studio` can verify required host tools early, starting with separate Godot and Blender checks that fit `init`, `wizard`, and future CLI flows.

## Scope

### In Scope
- Add composable tool checks for at least Godot and Blender.
- Expose a standalone CLI check entrypoint and shared internal APIs for reuse.
- Integrate selectable preflight checks into `init` and `wizard` without coupling tools together.
- Add tests for selection, execution, and failure reporting using command stubs.

### Out of Scope
- Version-manager/install automation for missing tools.
- Broad support for every engine/DCC; only the framework plus initial Godot/Blender checks.

## Capabilities

### New Capabilities
- `environment-tool-checks`: Register, select, and run independent host-tool checks from CLI flows.

### Modified Capabilities
- `installer-wizard`: Add explicit preflight behavior so required tool checks run before generation and fail with actionable guidance.

## Approach

Create a small checker framework with per-tool definitions (`godot`, `blender`) registered independently behind a common interface. Add `game-studio env-check` for direct diagnostics, and let `init`/`wizard` compose only the checks relevant to the current flow, selected packs, or requested tools. Keep policy separate from implementations so future tools can be added without changing command architecture.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/app.go` | Modified | Route new `env-check` command and help text. |
| `internal/cli/commands/init.go` | Modified | Run selected preflight checks during initialization. |
| `internal/cli/commands/wizard.go` | Modified | Add pre-generation tool validation in wizard flow. |
| `internal/cli/commands/env_check.go` | New | Standalone command for manual/CI diagnostics. |
| `internal/toolcheck/` | New | Registry, selector, runner, and tool-specific check implementations. |
| `internal/cli/commands/*_test.go` | Modified/New | Cover composable selection and error paths. |
| `openspec/specs/installer-wizard/spec.md` | Modified | Specify wizard preflight behavior. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Tool probing differs across installs/platforms | Med | Accept multiple version outputs and isolate probe logic per tool. |
| Over-coupling checks to current Godot flow | Med | Keep check registration tool-centric, with flow-specific selection rules. |
| External command tests become brittle | Med | Inject runners and use table-driven tests with stubs. |

## Rollback Plan

Remove `env-check` routing and preflight invocations, delete `internal/toolcheck/`, and revert the wizard spec delta. Existing `init`/`wizard` generation paths remain intact because checks are additive.

## Dependencies

- Existing CLI command architecture in `internal/cli/`.
- Host binary discovery/probing pattern established in `internal/persistence/engram.go`.

## Success Criteria

- [ ] Users can run separate Godot and Blender checks individually or together.
- [ ] `init` and `wizard` reuse the same checker framework instead of duplicating logic.
- [ ] Adding a new tool check requires a new registration/implementation, not command rewrites.
- [ ] Failures report which tool failed, why, and what command/path was expected.
