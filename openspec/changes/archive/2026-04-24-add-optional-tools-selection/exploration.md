## Exploration: add-optional-tools-selection

### Current State
The installer wizard currently collects connectors, packs, and generic `Tools` as free-form CSV in step 4, while MCP assistants are configured separately in step 10. The final wizard artifact and generated profile persist `tools` and MCP required/optional values, but there is no typed optional-tool domain model, no registry metadata for non-core integrations, and no install/launch policy.

Environment checks are already composable through `internal/toolcheck`: checks register stable IDs, aliases, severity, command expectations, and remediation. Wizard preflight maps selected resources/packs/tools to registered tool checks with token matching, then blocks only on required failures. This is a good pattern to reuse, but optional integrations like Engram Monitor and Metronous are not just host binaries: they may need config snippets, MCP/plugin wiring, service assumptions, ports, install hints, and privacy/telemetry consent.

Engram Monitor is a TypeScript/Vite dashboard for a local Engram HTTP server, requiring Node.js >=20, pnpm >=9, and Engram >=1.12.0 reachable at `127.0.0.1:7437`; it can start Engram during `pnpm dev`. Metronous is a Go/OpenCode telemetry stack with a daemon, plugin, MCP shim, local SQLite data, dashboard TUI, and optional ingest token; its installer patches OpenCode config and service setup, with Linux/macOS supported and Windows experimental.

### Affected Areas
- `internal/cli/commands/wizard.go` — wizard state, prompt sequence, summary, profile markdown, final artifact JSON, and selected preflight tools.
- `internal/cli/commands/wizard_test.go` — interactive/non-interactive flow fixtures, final artifact expectations, and separation between resources, MCPs, and optional tools.
- `internal/toolcheck/*` — reusable check registry can support optional probes, but install/config capabilities should not be forced into this package.
- `internal/cli/commands/tool_policy.go` — current token-to-check mapping may be reused for optional probes but is too implicit for integration policy.
- `internal/cli/commands/init.go` and `internal/cli/commands/generate.go` — existing MCP placeholders are hard-coded for Engram/Context7 and may need to surface selected optional integrations without making them mandatory.
- `openspec/specs/installer-wizard/spec.md` — wizard requirements currently define resource/MCP separation but not optional integrations.
- `openspec/specs/tool-check-*/spec.md` — existing registry/execution specs define composable checks and optional severity semantics that can be extended.

### Approaches
1. **Extend free-form tools CSV** — Add `engram-monitor` and `metronous` as suggested values in the existing Tools prompt and persist them in `tools`.
   - Pros: Smallest change; minimal UI disruption.
   - Cons: Conflates game development tools, MCPs, dashboards, and telemetry; no policy metadata; poor future asset-pipeline compatibility.
   - Effort: Low.

2. **Typed optional integration registry** — Introduce a small optional-tools/integrations registry with stable IDs, display labels, category, default selection policy, required consent flags, check IDs, install/config hints, and artifact metadata. Add a dedicated wizard step after MCP selection or after resources, then persist `optional_tools` separately from `tools` and `mcp`.
   - Pros: Keeps core decoupled and policy-driven; supports Engram Monitor/Metronous without hard-coding behavior into wizard flow; future asset pipeline adapters can register optional integrations with metadata.
   - Cons: Requires new domain types, specs, tests, and artifact schema changes.
   - Effort: Medium.

3. **Full installer actions for optional tools** — Add selection plus clone/install/configure/start behavior for Engram Monitor and Metronous.
   - Pros: More complete end-user automation.
   - Cons: High risk: external repos, OS/service differences, OpenCode config mutation, telemetry consent, Node/pnpm/systemd dependencies, and rollback complexity. Too much for the first policy-selection change.
   - Effort: High.

### Recommendation
Use Approach 2. Create an optional integration registry separate from `toolcheck`, and let registry entries reference tool checks and policy/config hints instead of performing installation. For this change, the wizard should only select, validate, summarize, and persist optional integrations; actual install/start/config mutations should remain explicit follow-up work.

Recommended initial IDs: `engram-monitor` and `metronous`. Suggested categories: `memory-viewer` for Engram Monitor and `telemetry` for Metronous. Suggested default: unselected unless non-interactive defaults are explicitly defined as safe; Metronous especially should require explicit opt-in because it captures agent telemetry/cost/session events.

### Risks
- Telemetry/privacy: Metronous selection implies event capture and OpenCode plugin wiring; require explicit consent language before any future install/config action.
- External dependency drift: Engram Monitor and Metronous install requirements may change; registry metadata should store URLs/hints, not assume commands as stable contracts.
- Artifact compatibility: adding `optional_tools` changes final artifact schema; tests should preserve existing `tools` and `mcp` behavior.
- Scope creep: install/start/service management would couple the core installer to OS-specific external tools; keep first change selection-only.
- Naming ambiguity: existing `Tools` means profile/game-dev tools, not optional ecosystem integrations; proposal should rename/clarify UX copy without breaking current fields.

### Ready for Proposal
Yes — propose a selection-only, registry-driven optional integrations layer that persists selected integrations separately from existing resources and MCPs, reuses toolcheck only for optional diagnostics, and defers external installation/configuration to later changes.
