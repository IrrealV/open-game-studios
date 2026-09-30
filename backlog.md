# Game-Studio Backlog

Last updated: 2026-09-29 (documentation checkpoint for the public snapshot)

This file is the curated, public-facing planning document for Open Game Studios
(OGS) / Game-Studio. Current capability and evidence live in
[docs/project-status.md](docs/project-status.md); the ordered work lives in
[docs/roadmap.md](docs/roadmap.md). Neither this file nor those pages is a
published release, an installed product, or a finished-v1 certification.

## Product vision

Game-Studio is **not** a one-shot game generator. The goal is to provide a disciplined workflow that helps people create genuinely enjoyable games: clear design intent, narrative/story coherence, art direction, implementation discipline, QA, playtesting, polish, and release readiness.

## Sources of truth

- **Repository documentation** — [docs/project-status.md](docs/project-status.md),
  [docs/roadmap.md](docs/roadmap.md), [docs/architecture.md](docs/architecture.md),
  and this file are the public planning truth once a public repository exists.
- **GitHub Issues (future)** — intended as the actionable backlog, each labeled
  by type/area/priority, **once** a public issue tracker is created. No public
  issue tracker, label set, or issue is configured today, and none of the legacy
  numbers below is a live public issue.
- **Engram / private memory** — optional and private. It is never required to
  contribute, and public continuity works from the files in this repository
  alone.
- **OpenSpec** — specs/design/tasks for active or completed SDD changes.

## Backlog and contribution policy

1. Contributions arrive as pull requests against the default branch; see
   [CONTRIBUTING.md](CONTRIBUTING.md). Pull-request review is the default for
   all changes, including small ones.
2. Do not commit directly to the default branch. Branches and pull requests are
   required for work of any size.
3. Commits MUST stay small, focused, and avoid mixing unrelated changes.
4. When a change relates to a tracked item, reference that item in the pull
   request description.
5. Issue closure is a human decision: a change does **not** close an issue
   automatically, and nothing in this document implies an automatic-close
   keyword or action.
6. `backlog.md` MUST be updated when roadmap order or process policy changes.
7. Visual work finishes before audio work.
8. Audio/music/SFX stay in their own workflow/change.
9. A public MVP release is blocked until OGS passes clean Windows, macOS, and
   Linux validation from zero required tools installed: install/configure from
   scratch, create at least two substantially different real games, deliberately
   provoke errors, and verify that missing tools, PATH, providers, engine,
   permissions, workflow lanes, visual/audio lanes, and smoke/verification
   failures produce clear, actionable, non-destructive diagnostics. A setup that
   only works on a prepared developer machine does not count.

## Label taxonomy (intended, not configured)

These labels describe the intended shape of a **future** public issue tracker.
**No labels are configured today, and this document does not create any issue.**
Every `legacy-*` ID below is a documentation label, not a live tracker.

### Type labels

- `type:feature` — new capability or workflow.
- `type:bug` — bug fix / defect.
- `type:optimization` — performance, workflow, or UX optimization.
- `type:security` — security hardening.
- `type:validation` — QA, compatibility, verification, smoke/acceptance validation.
- `type:workflow` — project process/backlog/workflow improvement.
- `type:docs` — documentation-only.
- `type:chore` — maintenance/tooling.

### Area labels

- `area:visual` — art direction, visual assets, ComfyUI, Blender.
- `area:audio` — music, SFX, ambience, audio import.
- `area:windows` — Windows compatibility and validation.
- `area:engine-pack` — Godot/Unity/UE5 packs.
- `area:process` — backlog, SDD, issue/PR workflow, production process.

### Priority labels

- `priority:high`
- `priority:medium`
- `priority:low`

When a future public tracker exists, issues SHOULD carry at least one `type:*`
label where possible.

## Current delivery lane — usable OGS v1

The user-selected implementation sequence is tracked with portable IDs in
[docs/roadmap.md](docs/roadmap.md): **V1-01 through V1-11**, proceeding one task
at a time with observed acceptance evidence, Linux/WSL first. The v1 plan is
approved and ongoing. This lane takes precedence over the historical ordering
below for current implementation, without deleting deferred capabilities or
changing the public-release gate. Legacy issue references below are traceability
only; their remote status was not rechecked except where a fresh scoped
observation is explicitly noted.

## Historical stabilization baseline (historical)

| Item | Status | Notes |
|---|---|---|
| legacy Hito 0 | Historical: implementation and review complete | A historical milestone label. It is **not** the current stopping point: the v1 plan is approved and ongoing. Legacy items `legacy-32`, `legacy-36`, `legacy-37`, and `legacy-30` remain deferred or unstarted, and none begins automatically. |

## Historical roadmap order (historical traceability)

Eleven rows, preserved in meaning from the original private plan. The order
labels are the original planning order (1, 2, 3, then 6 through 13); the gaps at
4 and 5 are historical and are **not** filled or invented here. Remote status
was freshly observed only where marked; the rest is historical evidence, not
live state.

| Order | Legacy ID | Type | Area | Priority | Capability and notes |
|---:|---|---|---|---|---|
| 1 | `legacy-29` | `type:optimization` | `area:process` | High | Optimize tool detection and validation UX for expanding setup workflows. Implemented selected-only preflight, validation severities, skipped/deferred reporting, and artifact validation metadata. **Observed CLOSED** (fresh scoped metadata check, 2026-09-29). |
| 2 | `legacy-32` | `type:optimization` | `area:visual` | High | Separate visual source lanes from production workflow lanes. Clarify source versus production lanes before generation providers/adapters. **Observed OPEN** (fresh scoped metadata check, 2026-09-29). |
| 3 | `legacy-30` | `type:feature` | `area:visual` | High | Add provider-native image generation workflow, originally scoped to OpenCode providers. Historical intent preserved; current and future provider work belongs to the Pi host, not to new OpenCode auth. No implementation is claimed. |
| 6 | `legacy-18` | `type:feature` | `area:visual` | High | Add ComfyUI visual workflow adapter — production lane for ComfyUI-driven refinement/execution, separate from the origin of assets or references. |
| 7 | `legacy-19` | `type:feature` | `area:visual` | High | Add Blender reference modeling workflow — production lane for Blender reference modeling and downstream refinement, not a universal source of assets. |
| 8 | `legacy-20` | `type:validation` | `area:visual` | Medium | Add Godot and Blender visual import validation: paths, scale, naming, formats, import metadata. |
| 9 | `legacy-21` | `type:validation` | `area:windows` | High | Run clean Windows validation for 2D and VR workflows — one acceptance lane; product support must remain broader than any single user test path. |
| 10 | `legacy-23` | `type:feature` | `area:process` | Medium | Add Stand Duel quality gate — product-facing dual/adversarial review gate, separate from internal Judgment Day. |
| 11 | `legacy-22` | `type:feature` | `area:audio` | Medium | Add audio asset pipeline workflow; starts only after visual workflows stabilize. |
| 12 | `legacy-24` | `type:feature` | `area:engine-pack` | Low | Add Unity engine pack, after the Godot-first flow matures. |
| 13 | `legacy-25` | `type:feature` | `area:engine-pack` | Low | Add Unreal Engine 5 engine pack, after the Godot-first flow matures. |

## Newly observed and deferred legacy items

| Legacy ID | Title | Observed status (metadata only, 2026-09-29) | Source progress |
|---|---|---|---|
| `legacy-36` | feat(installer): offer Godot installation during clean Windows setup | OPEN | No clean-Windows Godot installation is implemented. The installer backend (detect / plan / consent / execute) is source-accepted and the Linux/WSL G7 path remains pending; Windows installation stays future work. |
| `legacy-37` | feat(wizard): redesign installer as guided terminal UI | OPEN | The staged wizard and its plan-only preview are source-accepted with human visual acceptance, but that does not close a guided-terminal-UI redesign item. Local wizard PTY plus human acceptance is UI evidence, not closure of this feature request. |

These two items are explicit capabilities. Local source acceptance or UI
acceptance does not imply them. The remote query was deliberately scoped to
metadata for these IDs; no other remote status is asserted.

## Recently completed (legacy traceability)

Eleven rows preserved in meaning. Commit and pull-request references are
historical private traceability and are **not** public dependencies.

| Legacy ID | Notes |
|---|---|
| `legacy-38` | Harden workspace config validation for explicit setup-depth overrides — implemented by commits `190b737` and `fb90827`. Tests, isolated dogfood, and scoped Judgment Day re-judgment passed. |
| `legacy-26` | Fix Go coverage covdata toolchain warning — documented the tested-package POSIX coverage workflow in commit `00b3932`. Generic `go test ./... -cover` still reproduces the no-test-package warning; the toolchain is not claimed fixed. |
| `legacy-34` | Add phase/capability-aware model routing — implemented `model-routing/v1` as the authoritative metadata-only routing contract; generated artifacts keep legacy routing fields as compatibility/non-execution metadata only. |
| `legacy-31` | Optimize wizard optionals with presets and separate visual workflow lanes — added Minimal/Recommended/Full/Custom preset taxonomy and separated generic optionals from consent-sensitive integrations and workflow adapters. |
| `legacy-28` | Optimize installer wizard UX with staged setup flow — added staged first-run flow with use mode, engine pack, setup depth, review/generate/smoke/next steps, and compatibility fields. |
| `legacy-17` | Add core game design and narrative workflow — added `core-game-workflow/v1` with game intent, GDD slice, narrative modes, repair/change handoff, human approval gates, and no auto-approval. |
| `legacy-16` | Add art bible and asset spec workflow — added `visual-workflow/v1`, Art Bible/Asset Spec metadata, and readiness/audit guidance; merged via a legacy pull request. |
| `legacy-15` | Add persistent backlog and issue taxonomy — added durable `backlog.md`, issue taxonomy, and roadmap policy; merged via a legacy pull request. |
| `legacy-13` | Add visual asset pipeline metadata contract — metadata-only visual asset pipeline contract. |
| `legacy-11` | Add optional tools selection — added optional integrations selection. |
| `legacy-7` | feat(env-check): add composable environment tool checks — added Godot/Blender env checks. |

## Public release-readiness gate (future supported MVP)

Opening an unfinished source tree to contributors is **not** the same as
certifying a release. A public MVP release is blocked until OGS passes clean,
zero-tool installation plus at least two substantially different real game
workflows on Windows, macOS, and Linux, and until missing tools, PATH,
providers, engine, permissions, workflow lanes, visual/audio lanes, and
smoke/verification failures produce clear, actionable, non-destructive
diagnostics. A setup that only works on a prepared developer machine does not
count. This gate is separate from per-task source acceptance and is not claimed
met by the current source tree.
