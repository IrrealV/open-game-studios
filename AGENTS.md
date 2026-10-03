# AGENTS.md — repository-local agent instructions

Portable guidance for any coding agent working in this repository. It assumes no
specific harness, model, or private memory system.

## Start here

1. [README.md](README.md) — the current public entry point.
2. [docs/project-status.md](docs/project-status.md) — the authoritative capability checkpoint.
3. [docs/roadmap.md](docs/roadmap.md) — ordered work and acceptance boundaries.
4. [CONTRIBUTING.md](CONTRIBUTING.md) — the contribution flow.

## Current next task

The next concrete implementation task is **V1-03 — implement and check a small
Godot 3D task** (see the roadmap). It is a bounded, disposable 3D slice that
follows the accepted design cut; it is not another design interview, and it is
not full-platform certification.

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
