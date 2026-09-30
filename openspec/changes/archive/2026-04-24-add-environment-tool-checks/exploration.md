## Exploration: add-environment-tool-checks

### Current State
- The CLI entrypoint (`cmd/game-studio/main.go`) routes through `internal/cli/app.go` with commands: `init`, `generate`, `wizard`, `smoke`, `smoke-suite`.
- There is no explicit environment/tool preflight command today; installer behavior is effectively split between `init` (workspace bootstrap + skeleton validation) and `wizard` (14-step personalization + generation + smoke).
- Validation patterns are strict and deterministic:
  - `init` validates document and workspace artifacts via explicit field/key checks (`internal/cli/commands/init.go`).
  - `wizard` validates path safety and generated profile markers (`internal/cli/commands/wizard.go`).
  - `smoke` and `smoke-suite` validate generated artifact existence/content/JSON semantics (`internal/cli/commands/smoke.go`).
- Existing runtime dependency checks exist only for Engram CLI in persistence (`internal/persistence/engram.go` via `exec.LookPath("engram")`), which is the closest established pattern for tool availability checks.

### Affected Areas
- `internal/cli/app.go` — command routing/help text if a dedicated tool-check command is introduced.
- `internal/cli/commands/init.go` — natural place for early preflight during workspace initialization.
- `internal/cli/commands/wizard.go` — installer flow already has staged validations; tool checks can become an explicit step before generation.
- `internal/cli/commands/smoke.go` — optional host-level runtime checks if the team wants CI/runtime verification beyond artifact checks.
- `internal/cli/commands/init_test.go` and `internal/cli/commands/wizard_test.go` — current test style for table-driven validation and filesystem isolation (`t.TempDir()`) should be extended for tool-check scenarios.
- `internal/persistence/engram.go` — reference implementation for external binary availability checks (`exec.LookPath`) and user-friendly error wrapping.
- `openspec/specs/installer-wizard/spec.md` — likely requires a new requirement/scenario to formalize Godot/Blender preflight behavior in the 14-step flow.

### Approaches
1. **Dedicated `env-check` command + optional invocation from flows** — Introduce `game-studio env-check` that validates Godot/Blender installation/runtime, then call it (or shared function) from `init`/`wizard` as needed.
   - Pros:
     - Single responsibility and reusable check logic for humans and CI.
     - Backward-compatible rollout: users can run checks manually first.
     - Aligns with existing command-based architecture (`app.go` switch).
   - Cons:
     - Requires deciding enforcement level (warn vs fail) when called indirectly.
     - Adds one more user-facing command to maintain/docs/help text.
   - Effort: **Medium**

2. **Inline checks directly inside `init` and `wizard` only** — Add Godot/Blender checks as preflight inside existing flows without a separate command.
   - Pros:
     - Fastest path for early failure in the exact user journeys that matter.
     - Minimal command-surface expansion.
   - Cons:
     - Logic duplication risk across `init` and `wizard` unless carefully refactored.
     - Harder to run independently in CI or as a standalone diagnostic.
   - Effort: **Low/Medium**

3. **Extend `smoke-suite` to include host tool runtime checks** — Keep init/wizard unchanged and verify Godot/Blender in smoke-suite.
   - Pros:
     - Fits existing “verification command” semantics.
     - Good for CI gates.
   - Cons:
     - Too late for onboarding UX (fails after generation path decisions).
     - Conflicts with requirement to catch missing tools early in installer workflow.
   - Effort: **Low**

### Recommendation
Use **Approach 1** with a shared internal checker package and invoke it from both `init` and `wizard` as early preflight, while also exposing a direct `env-check` command.

Why this is best for this codebase:
- It matches current architecture (central command router + reusable command helpers).
- It avoids duplication and enables both interactive onboarding checks (early failure/warnings) and CI diagnostics.
- It creates a clean extension point for future optional tools (Engram Monitor, Metronous) without overloading `init`/`wizard` logic.

### Risks
- **Version/runtime probing variance**: Godot/Blender flags differ by install method/platform; runtime checks must tolerate acceptable output variations.
- **User friction from strict failures**: hard-failing on optional contexts may block users; behavior should be policy-driven (required vs optional tools).
- **Testability of external commands**: direct `exec.Command` usage can make tests brittle unless command execution is abstracted/injected.
- **Scope creep**: adding future optional tools too early can dilute this change; keep MVP scoped to Godot/Blender.

### Ready for Proposal
**Yes** — proceed to proposal with scope: (1) shared tool-check abstraction, (2) Godot/Blender checks with clear required semantics, (3) early invocation in `init` and `wizard`, (4) optional standalone `env-check` command, and (5) tests using `t.TempDir()` and controlled command stubs.
