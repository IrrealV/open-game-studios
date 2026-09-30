# Tasks: Add Environment Tool Checks

## Phase 1: Toolcheck Foundation

- [x] 1.1 Create `internal/toolcheck/types.go` with `Check`, `Metadata`, `Result`, status/severity types, selection options, and blocking-result helpers.
- [x] 1.2 Create `internal/toolcheck/command.go` with injectable `CommandRunner`, `CommandResult`, and production exec runner using context timeouts and Windows-safe error text.
- [x] 1.3 Create `internal/toolcheck/registry.go` with register/list/get/resolve APIs, unknown-tool failures, and independent default registrations for `godot` and `blender`.
- [x] 1.4 Create `internal/toolcheck/runner.go` to execute resolved checks independently, default non-fail-fast, optional fail-fast, and aggregate blocking failures.

## Phase 2: Initial Tool Checks

- [x] 2.1 Create `internal/toolcheck/godot.go` to probe `godot --version` then `godot4 --version`, returning attempted command/path and install/path remediation.
- [x] 2.2 Create `internal/toolcheck/blender.go` to probe `blender --version`, failing on missing binary and warning on runnable binary with incomplete version output.
- [x] 2.3 Keep Godot and Blender metadata/tool IDs separate; do not introduce shared “game tools” selection or fixed-pair behavior.
- [x] 2.4 Ensure all probe output and failures are readable on clean Windows shells, including PATH/configuration guidance and attempted command names.

## Phase 3: CLI and Flow Integration

- [x] 3.1 Create `internal/cli/commands/env_check.go` with repeated `--tool` selection, default initial set, formatted status output, and failing exit on blocking results.
- [x] 3.2 Modify `internal/cli/app.go` to route `env-check` and include concise help text.
- [x] 3.3 Modify `internal/cli/commands/init.go` to inject/stub env checking and run only tools derived from selected init requirements before writes.
- [x] 3.4 Modify `internal/cli/commands/wizard.go` to inject/stub env checking before generation, block on failures, and keep warnings visible in summary/state.

## Phase 4: Tests

- [x] 4.1 Add table-driven `internal/toolcheck` tests for registry listing, future registration, selected-only resolution, and unknown-tool results.
- [x] 4.2 Add stub-runner probe tests for Godot/Blender success, missing required tools, warning classification, attempted path, and remediation text.
- [x] 4.3 Add runner tests proving one failed check does not hide another selected result, plus fail-fast behavior when explicitly enabled.
- [x] 4.4 Add CLI command tests for `env-check --tool blender`, default checks, blocking exit, output classification, and no real Godot/Blender dependency.
- [x] 4.5 Add init/wizard tests using `t.TempDir()` and injected command runners to prove selected-tool-only preflight and no Godot+Blender coupling.
- [x] 4.6 Add Windows clean-room diagnostic tests for missing binaries/PATH guidance without requiring a Windows host.

## Phase 5: Alignment and Verification

- [x] 5.1 Review `openspec/specs/installer-wizard/spec.md`; no wording change required because current preflight, warning, and blocking-failure wording still matches the implementation.
- [x] 5.2 Update user-facing CLI/help docs if this repo has existing command documentation for `game-studio env-check`.
- [x] 5.3 Run `go test ./... -count=1` after implementation; tests must pass without Godot or Blender installed.
- [x] 5.4 External Judgment Day/adversarial verification completed; it found the Godot mixed-version probe issue, which was fixed and re-verified.
- [x] 5.5 Fix verification blocker: wizard preflight warnings are collected before summary output and covered by stdout ordering assertions.

## Phase 6: Judgment Day Hardening Batch

- [x] 6.1 Fix empty-selection semantics so `env-check` explicitly selects all by default while `init` and `wizard` explicitly select none when no relevant tool IDs are derived.
- [x] 6.2 Enforce Godot 4 version detection with table-driven coverage for Godot 3, Godot 4, malformed, and empty version output.
- [x] 6.3 Add registered tool alias support, including `godot4` → `godot`, without introducing substring false positives or Godot+Blender coupling.
- [x] 6.4 Preserve aggregated Godot probe diagnostics so an initial `godot` execution error is not hidden by a later `godot4` lookup failure.
- [x] 6.5 Add optional failure non-blocking semantics to reduce the risk of future optional checks accidentally blocking flows.
- [x] 6.6 Run `go test ./... -count=1` after hardening changes.

## Phase 7: Confirmed Judgment Day Follow-up

- [x] 7.1 Fix Godot probing to continue after non-4 or unparseable candidate output and succeed when a later candidate reports Godot 4.
- [x] 7.2 Preserve aggregated Godot diagnostics when all candidates fail.
- [x] 7.3 Add table-driven mixed-install coverage where `godot` reports 3.x and `godot4` reports 4.x.
- [x] 7.4 Make registry registration atomic when alias validation fails, with regression coverage.
- [x] 7.5 Run `go test ./... -count=1` after follow-up fixes.
