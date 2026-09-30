# Design: Add Environment Tool Checks

## Technical Approach

Add `internal/toolcheck` as a small composable framework: a registry owns independent checks, a resolver selects relevant checks, and an executor runs them with an injected command runner. CLI commands call the same package; policy for `env-check`, `init`, and `wizard` stays in command code so Godot and Blender never become a fixed pair.

## Architecture Decisions

| Decision | Alternatives considered | Rationale |
|---|---|---|
| New `internal/toolcheck` package with `Registry`, `Check`, `Runner`, and result types | Put probing directly in `internal/cli/commands`; attach checks to `templates.Registry` | Keeps environment checks reusable outside CLI parsing and makes future tools additive. `templates.Registry` remains pack metadata, not host probing. |
| Tool-specific files register independently (`godot.go`, `blender.go`) | One “game tools” check | Satisfies independent selection/execution and prevents hidden Godot+Blender coupling. |
| Inject `CommandRunner` rather than calling `exec.Command` directly | Global shell helpers or PATH checks only | Tests can stub external command behavior; probes can verify runnable binaries and parse output without brittle host dependencies. |
| Non-fail-fast default with blocking classification | Stop at first failure | Specs require unrelated checks to continue so users see all failures in one run. |

## Data Flow

```text
CLI args / init doc / wizard state
        │
        ▼
toolcheck.Resolve(selection, flow requirements)
        │ unknown tool -> failed result before probes
        ▼
toolcheck.Runner.Execute(resolved checks, CommandRunner)
        │
        ├─ godot check: LookPath/Run `godot --version`
        └─ blender check: LookPath/Run `blender --version`
        ▼
formatted results -> CLI output / wizard summary / blocking error
```

`env-check` defaults to the registered supported set (`godot`, `blender` initially) unless `--tool` is provided. `init` and `wizard` derive requirements by matching selected profile resources, packs, tools, or primary-engine values against registered tool IDs; adding a future registered tool allows those flows to select it without command rewrites. Blender is not forced unless selected resources require it. `wizard` runs this preflight before step 13 generation and stores warnings in state for summary output.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/toolcheck/types.go` | Create | IDs, metadata, severity/status, result, options, and interfaces. |
| `internal/toolcheck/registry.go` | Create | Register/list/get/resolve checks; default registry registers Godot and Blender separately. |
| `internal/toolcheck/runner.go` | Create | Executes selected checks, supports fail-fast option, aggregates blocking errors. |
| `internal/toolcheck/command.go` | Create | `CommandRunner` plus production `exec` implementation using context timeouts. |
| `internal/toolcheck/godot.go` | Create | Probes `godot --version`, fallback `godot4 --version`; missing required tool fails with install/path guidance. |
| `internal/toolcheck/blender.go` | Create | Probes `blender --version`; incomplete output warns if binary is runnable. |
| `internal/cli/commands/env_check.go` | Create | Parses repeated `--tool`, runs selected checks, prints statuses, returns error on blocking failures. |
| `internal/cli/app.go` | Modify | Route `env-check` and update help. |
| `internal/cli/commands/init.go` | Modify | Add optional checker dependency and run resolved init preflight after doc validation, before writes. |
| `internal/cli/commands/wizard.go` | Modify | Add preflight step before generation; persist warnings in wizard state and summaries/artifacts. |
| `internal/**/*_test.go` | Create/Modify | Unit coverage for selection, execution, CLI, init, and wizard preflight behavior. |

## Interfaces / Contracts

```go
type CommandRunner interface { LookPath(string) (string, error); Run(ctx context.Context, name string, args ...string) (CommandResult, error) }
type Check interface { Metadata() Metadata; Run(context.Context, CommandRunner) Result }
type Metadata struct { ID, Name string; Commands []string; Required bool; Remediation string }
type Result struct { ToolID string; Status Status; Reason, Attempted, Remediation string }
```

Command inputs should expose injectable fields where tests need stubs, e.g. `EnvChecker *toolcheck.Runner` or `CommandRunner toolcheck.CommandRunner`; nil uses defaults.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | registry registration, unknown selections, selected-only resolution | Table-driven tests. |
| Unit | Godot/Blender probes and result classification | Stub `CommandRunner`; no real binaries. |
| Integration | `env-check`, `init`, `wizard` preflight and output/error behavior | Existing command tests with `t.TempDir()`, captured stdout, injected runner. |

Validate with `go test ./... -count=1`; do not require Godot or Blender installed.

## Migration / Rollout

No data migration required. Behavior is additive: new `env-check` command plus preflight before writes/generation. Rollback removes routing and preflight calls; existing generation paths remain intact.

## Open Questions

None.
