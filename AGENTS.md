# AGENTS.md — repository-local agent instructions

Portable guidance for any coding agent working in this repository. It assumes no
specific harness, model, or private memory system.

## Start here

1. [README.md](README.md) — the current public entry point.
2. [docs/project-status.md](docs/project-status.md) — the authoritative capability checkpoint.
3. [docs/roadmap.md](docs/roadmap.md) — ordered work and acceptance boundaries.
4. [CONTRIBUTING.md](CONTRIBUTING.md) — the contribution flow.

## Current next task

The next production task is **V1-05 — produce a real Blender asset and export**
(see the roadmap). First reconcile the approved visual inputs and prepare only
the resources and coordinated handoff needed for that unit. The disposable MCP
cube is execution-foundation evidence, not the V1-05 production asset.

**V1-03 is historically accepted at disposable-fixture mechanical and human
playtest scope.** Its recorded headless result remains **20/21**: the diagonal
verifier has a known expected-value defect. Do not erase that failure or treat
the scene as unimplemented. The verifier correction is a separate follow-up,
not a prerequisite unless it demonstrably blocks the selected production task.

**V1-04 is historically accepted at approved visual-metadata scope:** Art Bible,
three Asset Specs and an Asset Audit. This does not approve generated assets or
certify a production-role handoff. The Art Bible records approval but retains
stale pending prose; the committed audit retains its pre-approval state. Its
primitive-only constraints also need a bounded production-scope decision before
claiming they authorize an imported Blender asset. Preserve the approval record
and distinguish historical reports from freshly executed checks.

**V1-02 is accepted at bounded source-local scope.** The read-only design role
was actually invoked once for a creation request and returned an eight-section
`gdd-slice` creation proposal (a `coregame.DraftRequest`), not an approval or an
executable plan; the `brief` CLI then produced the canonical pending draft from
that proposal and accepted the downstream consistency of an observed human
approval of that exact One Small Reach creation brief. The completed run built
the CLI once, produced one pending draft once, and accepted one approved
decision once, with no timeout.

The pending draft kept `Status: draft`, `ApprovalState: pending_human_approval`,
and `AutoApproved: false`; those fields are never approval. The CLI acceptance
is **consistency validation only**: it does not authenticate a human, capture
consent, or grant execution authority. The library and CLI support creation,
direct-phase, change, and repair routes, but only this one live creation journey
was observed; the other routes rest on historically accepted source and
fixtures, not on four live invocations. Do not treat a draft `Status`, an
`ApprovalState` field, or a synthetic test decision as approval.

## Working rules

- Read the applicable contracts before changing behavior: `internal/workflows/coregame/contract.go`,
  [docs/core-game-briefs.md](docs/core-game-briefs.md),
  [docs/generated-artifacts.md](docs/generated-artifacts.md), and the skill
  sources under `skills/`.
- Keep changes bounded to the task's stated scope. This repository often carries
  deliberate uncommitted work; preserve unrelated working-tree changes.
- Produce truthful evidence. Separate what you observed from what you assume,
  and label historical evidence as historical.
- Run only the checks the task authorizes, exactly as written, and report every
  failure, skip, and pending check.
- Stop on unexpected mutation, an out-of-scope write, or a failing required
  check instead of working around it.
- Do not rewrite a human's personal state, local configuration, or private
  memory, and do not push, publish, or mutate remotes unless explicitly asked.
- Do not add secrets, credentials, personal data, or private memory content to
  the repository.
- Do not self-approve work, and do not paste internal reviewer tokens, logs, or
  private process identifiers into repository files.

## Not required

Working here does not require installing this project's harness or memory
system, using a specific model, or reading private task or memory artifacts.
Public continuity must be possible from the files in this repository alone.
