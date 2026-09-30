# Design: add-installer-personalization-wizard

## Technical Approach

Implement a 14-step CLI wizard that guides the user through creating a customized `Game-Studio` profile. It leverages the `opencode` package to snapshot connected providers, captures routing policies (tiers, phases, roles), and emits layered artifacts. It uses a hybrid persistence strategy, writing to the filesystem and logging to Engram.

## Architecture Decisions

### Decision: State Model & Collection

**Choice**: A sequential 14-step CLI wizard accumulating data into a single `wizardState` struct.
**Alternatives considered**: A declarative configuration file approach, or a TUI (Bubbletea, explicitly deferred and not part of the current bufio-based runtime flow).
**Rationale**: A sequential wizard lowers the barrier to entry while remaining flexible. A TUI was deferred for simplicity, and sequential steps naturally guide a user through OpenCode's concepts (packs, connectors, tools, MCPs, models) without overwhelming them upfront.

### Decision: Provider Discovery

**Choice**: Only discover OpenCode-connected providers via `opencode.ListConnectedProviders()`. Show helper text restricting the view.
**Alternatives considered**: Ask the user to input API keys or credentials directly.
**Rationale**: Enforces the OpenCode-only constraint. It prevents the wizard from becoming a credential manager, offloading that responsibility to OpenCode's environment.

### Decision: Artifact Output & Layering

**Choice**: Emit a human-readable profile Markdown (`.opencode/profiles/<name>.md`), a machine-readable JSON artifact (`.game-studio/generated/wizard/final.artifact.json`), and modify `openspec/config.yaml`.
**Alternatives considered**: Storing everything in a single JSON or YAML file.
**Rationale**: Layering allows human editing/reading in `.opencode/profiles` while the machine-readable `.artifact.json` serves as a stable source of truth for generation pipelines. Writing a managed block to `openspec/config.yaml` seamlessly integrates the wizard with the OpenSpec workflows.

### Decision: Model Routing Policy

**Choice**: Default to a 'balanced' preset with `fast`, `balanced`, and `deep` tiers, plus optional overrides per-phase (SDD) and per-role.
**Alternatives considered**: Explicitly asking for a model per tool or step.
**Rationale**: Tier-based routing provides sensible defaults out-of-the-box, but advanced/expert complexity levels still expose fine-grained control for power users without overwhelming simple workflows.

## Data Flow

    CLI Input (14 Steps)
           │
           ▼
    wizardState Struct ───────▶ opencode (Snapshot Providers)
           │
           ▼
    Artifact Generation
           │
           ├─▶ .opencode/profiles/{name}.md (Human readable)
           ├─▶ .game-studio/generated/wizard/final.artifact.json (Machine)
           ├─▶ openspec/config.yaml (Managed Block)
           └─▶ Engram (Write-Through)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/cli/commands/wizard.go` | Modify/Create | Contains the 14-step wizard logic, state struct, and artifact generation. |
| `.opencode/profiles/{profile}.md` | Create (Generated) | Emitted Markdown profile summary. |
| `.game-studio/generated/wizard/final.artifact.json` | Create (Generated) | Full machine-readable wizard state. |
| `openspec/config.yaml` | Modify | Appends a managed block tracking the wizard's output. |

## Interfaces / Contracts

```go
type wizardState struct {
	Complexity      string
	StartingPoint   string
	ProfileName     string
	Connectors      []string
	Packs           []string
	Tools           []string
	Providers       []opencode.ConnectedProvider
	ProviderWarning string
	Snapshot        opencode.ModelSnapshot
	Policy          routing.Policy
	MCPRequired     []string
	MCPOptional     []string
	Confirmed       bool
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | State validation & Normalization | Test `normalizeComplexity`, `splitCSV`, `parseOverride` functions independently. |
| Integration | Artifact Generation | Run wizard with non-interactive flag, verify generated files, paths, and JSON schema. |
| E2E | Wizard Flow | Simulate CLI inputs for the 14 steps, verify standard output and final artifact match expectations. |

## Migration / Rollout

No migration required for existing setups; the wizard operates on a new pathway. The `openspec/config.yaml` managed block safely appends without destroying existing user configs.

## Open Questions

- None
