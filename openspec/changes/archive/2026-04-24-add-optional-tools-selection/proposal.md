# Proposal: Add Optional Tools Selection

## Intent

Add a selection-only optional integrations layer so the wizard can offer ecosystem tools like Engram Monitor and Metronous without mixing them into game-dev `tools`, MCP assistant configuration, or host tool checks. Metronous selection must explicitly represent telemetry/privacy consent.

## Scope

### In Scope
- Add a registry-driven optional integrations model with stable IDs, labels, categories, selection defaults, check references, hints, and consent metadata.
- Add wizard selection, summary, and artifact persistence for initial IDs `engram-monitor` and `metronous`.
- Preserve existing `tools` and `mcp` outputs while adding compatible optional-tool selections.

### Out of Scope
- Installing, cloning, configuring, starting, or managing external services.
- Mutating OpenCode, MCP, systemd, launchd, or shell configuration for selected integrations.
- Reclassifying existing game-dev tools or MCP assistants as optional tools.

## Capabilities

### New Capabilities
- `optional-tools-selection`: Registry, wizard semantics, artifact schema, and consent rules for optional integrations.

### Modified Capabilities
- `installer-wizard`: Add a distinct optional integrations selection stage and output field.
- `tool-check-cli-integration`: Allow optional integrations to reference diagnostics without making checks mandatory.

## Approach

Use a small optional integrations registry separate from `internal/toolcheck`. Registry entries expose policy metadata and optional diagnostic check IDs; wizard code renders/selects entries and persists selections under a new optional-tools field. `metronous` requires explicit opt-in consent copy because it represents telemetry capture; `engram-monitor` remains a local memory-viewer integration. Existing `tools` and `mcp` fields remain unchanged.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/commands/wizard.go` | Modified | Selection step, state, summary, final artifact/profile output. |
| `internal/cli/commands/wizard_test.go` | Modified | Interactive and artifact compatibility coverage. |
| `internal/cli/commands/tool_policy.go` | Modified | Map optional integrations to non-blocking diagnostics where applicable. |
| `internal/toolcheck/*` | Modified | Reuse checks only as diagnostics; no install/config policy. |
| `.game-studio/generated/wizard/final.artifact.json` | Modified | Add optional-tool selections compatibly. |
| `.opencode/profiles/game-studio.md` | Modified | Surface selected optional integrations as metadata/hints only. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Telemetry consent ambiguity | Med | Require explicit Metronous opt-in wording and persisted consent semantics. |
| Artifact schema breakage | Med | Add new field; do not alter existing `tools` or `mcp`. |
| Scope creep into installers | Med | Keep registry metadata/hints declarative; defer actions. |
| Dependency drift | Low | Store URLs/hints, not authoritative install commands. |

## Rollback Plan

Remove the optional integrations registry, wizard step, tests, and new artifact/profile field. Existing `tools` and `mcp` artifacts remain valid because they are not changed.

## Dependencies

- Existing wizard generation and `internal/toolcheck` diagnostics.

## Success Criteria

- [ ] Wizard offers `engram-monitor` and `metronous` separately from tools and MCPs.
- [ ] Metronous cannot be selected without explicit telemetry/privacy consent semantics.
- [ ] Generated artifacts preserve existing `tools`/`mcp` behavior and carry optional selections compatibly.
- [ ] No external install, service, or config mutation occurs.
