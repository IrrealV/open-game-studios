# Tasks: Add Installer Personalization Wizard

> Note: Runtime wizard UX follows the richer **14-step sequence (0..14)**. Phase sections below are implementation workstreams, not a collapsed runtime wizard flow.

## Phase 1: Foundation & Setup (OpenCode-First)

- [x] 1.1 **Discovery**: Locate OpenCode provider configuration in `~/.local/share/opencode/auth.json` (or platform equivalent).
- [x] 1.2 **Discovery**: Verify schema for listing only connected providers.
- [x] 1.3 Implement `internal/opencode/discovery.go` to filter and list connected providers.
- [x] 1.4 Implement `internal/opencode/models.go` with `balanced` preset, tiers, and role/phase routing logic.

## Phase 2: Wizard Skeleton & Branching

- [x] 2.1 Scaffold `internal/cli/commands/wizard.go` with a `bufio` prompt controller for multi-step flow.
- [x] 2.2 Implement Complexity Selection step (Simple / Advanced / Expert) with state-based branching.
- [x] 2.3 Implement "Start from Scratch" flow: allow wizard to run and initialize a workspace without an existing repo.
- [x] 2.4 Implement Profile Naming step: allow naming/renaming the single profile (default: "Game-Studio").

## Phase 3: Provider & Model Configuration

- [x] 3.1 Implement Provider Discovery screen (UI: show ONLY connected providers; helper text "Solo se muestran los providers conectados a opencode").
  - [x] 3.1.a Handle auth-present-but-empty provider discovery with non-fatal fallback routing.
- [x] 3.2 Implement Provider-Aware Model Snapshot logic: capture available models for the profile.
- [x] 3.3 Implement "Balanced" preset screen: show explanations for the default model choice.
- [x] 3.4 Implement Model Routing UI: Simple (preset), Advanced (per-role), Expert (per-phase overrides).

## Phase 4: Resource Selection & MCPs

- [x] 4.1 Implement Connector/Pack/Tool selection screen (separate from MCPs; Godot-first).
- [x] 4.2 Implement MCP Selection: Force Engram/Context7; allow selection of optional extras.
- [x] 4.3 Implement CCGS-semantics layered packaging for the selected resources.

## Phase 5: Artifact Generation & Persistence

- [x] 5.1 Implement Artifact Generation: Create `.opencode/profiles/{name}.md` from wizard state.
- [x] 5.2 Implement Hybrid Persistence: Save generated profile and `openspec/config.yaml` to filesystem and Engram.
  - [x] 5.2.a Engram write-through for wizard final artifact generation via local `engram` CLI.
  - [x] 5.2.b Persist generated profile + OpenSpec config artifacts with same hybrid write-through contract.
- [x] 5.3 Wire the `wizard` command to the main CLI application.

## Phase 6: Validation & Future-Proofing

- [x] 6.1 Implement Smoke Validation: Verify generated artifacts for structural integrity.
- [x] 6.2 Add non-MVP placeholders for future sync/refresh functionality.
- [x] 6.3 Add focused Go tests for provider discovery fallback and wizard no-provider continuation path.
  - [x] 6.3.b Add runtime integration tests for full 14-step flow, cancellation boundary, profile rename artifacts, MCP/resource separation, and deferred sync metadata.
