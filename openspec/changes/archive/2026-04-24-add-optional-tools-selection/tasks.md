# Tasks: Add Optional Tools Selection

## Phase 1: Foundation / Registry

- [x] 1.1 Create `internal/integrations/types.go` with `Integration`, `ConsentRequirement`, `OptionalIntegrationSelection`, and lookup/list contracts.
- [x] 1.2 Create `internal/integrations/registry.go` registering `engram-monitor` and `metronous` with stable IDs, labels, categories, defaults, hints, diagnostics, and consent metadata.
- [x] 1.3 Add table-driven tests in `internal/integrations/registry_test.go` for catalog contents, stable lookup, default selection flags, hints, diagnostics, and Metronous consent metadata.

## Phase 2: Diagnostic Policy

- [x] 2.1 Modify `internal/cli/commands/tool_policy.go` to resolve optional integration diagnostic IDs separately from required tool checks.
- [x] 2.2 Ensure optional diagnostics are warning-only, ignored when missing/unregistered, and never enter blocking preflight or `env-check` defaults.
- [x] 2.3 Add focused tests in `internal/cli/commands/wizard_test.go` or a new `tool_policy_test.go` covering optional diagnostic exclusion from required checks.

## Phase 3: Wizard Implementation

- [x] 3.1 Modify `internal/cli/commands/wizard.go` wizard state to store optional integration selections and consent maps without changing existing `tools` or `mcp` fields.
- [x] 3.2 Add the optional integrations step after MCP selection, rendering registry entries by stable ID and keeping non-interactive defaults empty.
- [x] 3.3 Add explicit Metronous telemetry/privacy consent handling; decline or missing consent MUST omit `metronous` from selections.
- [x] 3.4 Update wizard summaries to show selected optionals and consent states as metadata/hints only, never as installed/started services.

## Phase 4: Artifact / Profile Output

- [x] 4.1 Modify `.game-studio/generated/wizard/final.artifact.json` generation to add `optional_integrations.selected` while preserving existing `tools` and `mcp` output.
- [x] 4.2 Modify `.opencode/profiles/game-studio.md` generation to include optional integration metadata/hints only, with no install or service-management instructions.

## Phase 5: Tests / Verification

- [x] 5.1 Add wizard tests for selecting `engram-monitor`, accepting Metronous consent, and rejecting/missing Metronous consent.
- [x] 5.2 Add artifact/profile tests asserting `tools` and `mcp` remain unchanged and Metronous consent persists as `telemetry_privacy: true` only when granted.
- [x] 5.3 Add regression tests for no external install/clone/config/start/stop actions and optional diagnostics not blocking generation.
- [x] 5.4 Verify with `go test ./... -count=1`.
