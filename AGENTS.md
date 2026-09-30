# AGENTS.md — repository-local agent instructions

Portable guidance for any coding agent working in this repository. It assumes no
specific harness, model, or private memory system.

## Start here

1. [README.md](README.md) — the current public entry point.
2. [docs/project-status.md](docs/project-status.md) — the authoritative capability checkpoint.
3. [docs/roadmap.md](docs/roadmap.md) — ordered work and acceptance boundaries.
4. [CONTRIBUTING.md](CONTRIBUTING.md) — the contribution flow.

## Current next task

The open implementation task is **V1-02 — make design and its existing contracts
executable** (see the roadmap). After the current publication-documentation
work, the next concrete step is a bounded design-role handoff followed by an
**observed** human approval gate.

The core-game brief library and the `brief` CLI are already accepted at source
scope. The design role file exists and is host-listable, but it has not been
invoked or independently accepted. Do not treat a draft `Status`, an
`ApprovalState` field, or a synthetic test decision as approval, and do not
describe V1-02 as complete.

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
