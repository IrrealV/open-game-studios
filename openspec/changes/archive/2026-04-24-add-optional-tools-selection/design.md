# Design: Add Optional Tools Selection

## Technical Approach

Add a small registry-backed optional integrations layer beside, not inside, existing wizard `Tools`, MCP selection, and `internal/toolcheck`. The wizard will collect selected integration IDs after MCP selection, require explicit Metronous telemetry/privacy consent, run only non-blocking diagnostics referenced by registry metadata, and add a new artifact/profile section without changing existing `tools` or `mcp` fields.

## Architecture Decisions

| Decision | Choice | Alternatives considered | Rationale |
|---|---|---|---|
| Integration model | Create `internal/integrations` with registry/types for optional integrations | Extend free-form `Tools`; fold into MCPs; put metadata in `toolcheck` | Keeps concepts separated: game-dev tools, MCP assistants, engine diagnostics, and optional ecosystem integrations each have their own policy boundary. |
| Selection semantics | Default no optional integrations in non-interactive mode; interactive CSV by stable IDs | Preselect Engram Monitor; suggest Metronous by default | Selection-only must be explicit, and Metronous telemetry cannot be silently enabled. |
| Consent | Registry entry declares consent requirements; wizard blocks Metronous selection unless consent is answered yes and persists that consent | Hard-code Metronous in artifact writer; store only selected IDs | Centralized metadata prevents future consent-bearing integrations from scattering privacy logic through wizard code. |
| Diagnostics | Registry entries may reference `toolcheck` IDs; wizard runs those as optional warnings only when selected and registered | Make missing diagnostics blocking; add install checks | This reuses existing checks without turning optional integrations into host prerequisites or service managers. |
| Artifact compatibility | Add new `optional_integrations` field and profile section; do not mutate `tools`/`mcp` schema | Rename `tools`; nest all selections into a new schema | Existing consumers continue reading current fields while newer consumers can opt into the added metadata. |

## Data Flow

```text
internal/integrations registry
        │
        ▼
wizard optional integrations step ──→ consent prompt for metronous
        │                                  │
        ├──→ optional diagnostic check IDs ─┴──→ toolcheck warnings only
        │
        └──→ wizard state ──→ profile markdown + final artifact JSON
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/integrations/types.go` | Create | Define `Integration`, `ConsentRequirement`, `Selection`, and registry lookup/list contracts. |
| `internal/integrations/registry.go` | Create | Register `engram-monitor` and `metronous`, including labels, categories, hints, consent metadata, and diagnostic check IDs. |
| `internal/cli/commands/wizard.go` | Modify | Add state, wizard step after MCPs, summary/profile/artifact output, and consent handling. Step numbering becomes a 15-step flow. |
| `internal/cli/commands/tool_policy.go` | Modify | Add helper to resolve optional integration diagnostic check IDs without failing on absent checks. |
| `internal/cli/commands/wizard_test.go` | Modify | Cover separation from resources/MCPs, Metronous consent, non-interactive defaults, artifact compatibility, and profile hints. |
| `.game-studio/generated/wizard/final.artifact.json` | Modify | Generated output gains `optional_integrations` only. |
| `.opencode/profiles/game-studio.md` | Modify | Generated profile gains optional integration metadata/hints only. |

## Interfaces / Contracts

```go
type Integration struct {
    ID, Label, Category, Description string
    DefaultSelected bool
    DiagnosticToolIDs []string
    Hints []string
    Consent []ConsentRequirement
}

type ConsentRequirement struct {
    ID, Label, RequiredText string
}

type OptionalIntegrationSelection struct {
    ID string `json:"id"`
    Consent map[string]bool `json:"consent,omitempty"`
}
```

Artifact addition:

```json
"optional_integrations": {
  "selected": [{"id":"metronous","consent":{"telemetry_privacy":true}}]
}
```

If no integrations are selected, emit `selected: []` or omit only via `omitempty` if tests confirm current artifact readers tolerate absence; never alter existing fields.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Registry IDs, metadata, consent requirements, diagnostic resolution | Table-driven Go tests; fake toolcheck registry. |
| Wizard flow | Interactive selection, Metronous consent rejection/acceptance, non-interactive none | Direct step tests with `bufio.Reader`; full `RunWizard` fixture tests using `t.TempDir()`. |
| Artifact/profile | Existing `tools`/`mcp` unchanged and new output present | JSON unmarshal assertions and markdown token checks. |

## Migration / Rollout

No data migration required. This only adds optional output fields and generated markdown sections. Rollback removes the registry, wizard step, and new output fields.

## Open Questions

- [ ] None.
