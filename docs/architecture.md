# Architecture

This page maps the current source tree and its boundaries. It documents what
exists; it does not introduce a new architecture. For current capability status
see [project-status.md](project-status.md).

## Layers

| Layer | Owner | Responsibility |
|---|---|---|
| Domain | OGS | Game-production contracts, phases, briefs, and studio responsibilities. |
| Host | Pi | Sessions, package/skill loading, authentication, and model selection. |
| Coordinator | Gentle Shell | Orchestration, delegation, change tracking, and the review boundary. |
| Persistence | OpenSpec + Engram | Durable change artifacts and project memory (Engram is a separate companion). |
| Engine | Godot (first) | Actual gameplay execution and project files. |

OGS owns the domain and does not redefine Pi authentication, Gentle Shell
orchestration, or the native review authority.

## Source module map

| Path | Role |
|---|---|
| `cmd/game-studio/main.go` | CLI entry point; delegates to `internal/cli`. |
| `internal/cli/app.go` | Command dispatch and top-level help. |
| `internal/cli/commands/` | Command implementations: `init`, `env-check`, `generate`, `wizard`, `smoke`, `smoke-suite`, `brief`. |
| `internal/templates/` | Embedded profile templates and the template registry. |
| `internal/piinstall/` | Prerequisite detection, fingerprint-bound plan, consent, execution, download/archive handling, and destination ownership. |
| `internal/persistence/` | Engram CLI write-through and the placeholder used outside wizard execution. |
| `internal/toolcheck/` | Host tool diagnostics for Godot, Blender, and selected tools. |
| `internal/workdoc/` | Working-document rendering. |
| `internal/workflows/coregame/` | Core-game contract plus the brief library (`BuildDraft`, `ValidateDownstream`). |
| `internal/workflows/modelrouting/` | Phase/capability model-routing metadata contract. |
| `internal/routing/` | Routing policy derived from resolved setup inputs. |
| `internal/assets/` | Visual asset metadata catalog. |
| `internal/integrations/` | Optional integration registry (never implicitly enabled). |
| `internal/opencode/` | Legacy OpenCode discovery/snapshot code, retained as history. |
| `skills/` | Canonical Pi skill sources and their Go embedding (`embed.go`). |
| `.pi/agents/` | Project-local read-only agent role definitions. |
| `tests/` | Node checkers, fixtures, and the readiness/consent dogfood harness. |
| `testdata/` | Godot fixtures used by the Node checks. |
| `docs/` | Canonical documentation. |

## Installer vs runtime

The installer and the runtime are deliberately separate:

- The **installer** (`piinstall` + the `wizard` command) detects prerequisites,
  builds an immutable plan, requires an explicit fingerprint-bound approval, and
  then executes confirmed actions. It reuses compatible existing installations
  and refuses to overwrite unknown or conflicting ones. A plan preview performs
  no installation.
- The **runtime** is the user's Pi host plus Gentle Shell. OGS neither bundles
  nor silently installs them, and it does not create a second orchestrator. A
  completed install reports a launch command; it does not edit the shell profile
  or `PATH`.

## Data and control ownership

| Data | Owner | Notes |
|---|---|---|
| Authentication, models, providers | Pi | OGS does not inspect credentials or model inventory. |
| Orchestration, delegation, review boundary | Gentle Shell | OGS reuses the existing coordinator. |
| Workspace configuration | `.game-studio/workspace.config.json` | Confirmed local preferences; the setup-preset authority. |
| Workspace seed | `.game-studio/workspace.manifest.json` | Workspace identity/topology. |
| Generated profile artifacts | `profiles/game-studio/generated/`, `.game-studio/generated/` | Local, disposable unless policy says otherwise. |
| Durable change artifacts | `openspec/` | Specs, design, and tasks. |
| Project memory | Engram companion | Separate; the repository does not embed a database. |

Generated output never silently becomes a configuration source. The precedence
and classification rules live in [generated-artifacts.md](generated-artifacts.md).

## Draft vs decision (core-game briefs)

The brief library has two distinct steps and one deliberately narrow trust
model:

1. `BuildDraft` turns a structured request into a **pending** draft. It always
   produces `Status: draft`, `ApprovalState: pending_human_approval`, and
   `AutoApproved: false`, and it never invents design prose or approvals.
2. `ValidateDownstream` proves that a draft is canonically coherent, the
   referenced bytes are current, the phase template is complete for handoff, and
   a supplied recorded decision is bound to the exact draft revision and target.

`ValidateDownstream` is **consistency validation only**. It does not
authenticate a human, capture consent, or grant execution authority. A synthetic
decision used in tests is a fixture, not approval. The CLI is a thin adapter
over this library and follows the same limitation. See
[core-game-briefs.md](core-game-briefs.md) for the field-level detail.

Caller-supplied text is treated as data, never as document structure: section
bodies, the request, and reference locators are rendered as fenced literal
blocks, and machine-owned sections are reconstructed from typed fields.

## Embedding vs role discovery

Two different mechanisms exist and should not be conflated:

- **Embedding** — the Go installer embeds the canonical skill sources (`skills/`)
  so it can copy them into a destination without depending on a developer
  checkout. The Node package manifest (`package.json`) separately publishes the
  same skill directories as Pi skills.
- **Role discovery** — the host discovers project-local roles under
  `.pi/agents/` at runtime. These role files are read-only definitions that read
  coordinator-supplied paths. A source-checkout path is not an installed-pack
  path, and role discovery is not the same as a skill being loaded or a role
  being invoked.

## Boundaries to preserve

- Do not add a second orchestrator, provider system, or permission framework.
- Do not rewrite the native review authority or treat its outcome as local.
- Do not let generated metadata become an implicit configuration or approval
  source.
- Do not treat planning text, a draft, or a handoff as execution or consent.
