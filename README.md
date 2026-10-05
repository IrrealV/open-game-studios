# Open Game Studios

Open Game Studios (OGS) is a human-led, Godot-first game-production workspace
for Pi and Gentle Shell. It combines six on-demand studio responsibilities
(design, art direction, Blender, Godot, audio, QA) with explicit approval gates
and truthful provenance.

**Current components:**
- Go CLI (`game-studio`) for workspace bootstrap, diagnostics, and staged wizard
- Visual wizard with plan-only preview (Bubble Tea UI, human-accepted)
- Native Pi MCP foundation: explicit create-only configuration and local launchers
- Core-game brief library: design → draft → consistency validation
- Two agent roles (design specialist, Godot specialist) for read-only planning
- Two Pi companion skills for brief workflows and Godot changes
- Test fixtures: 2D movement with human playtest; 3D prototype in progress

OGS is **not** a finished v1, not an installed product, and not a one-prompt
game generator. The first-party source is MIT-licensed (see
[Licensing](#licensing)).

**Repository:** <https://github.com/IrrealV/open-game-studios>

This repository is public and open for contributions as a work in progress.
No installable release exists yet; see [Project status](docs/project-status.md)
for current capabilities and evidence levels.

## Development

- **Branch protection:** `main` requires PR review; no force-push or direct commits
- **CI:** Go workflow runs `go build` and `go test ./...` on all PRs
- **Review policy:** See [CONTRIBUTING.md](CONTRIBUTING.md) for the full flow

## Current reality

**What exists today:**

- **Installer/generator (Go):** the `game-studio` CLI bootstraps a workspace,
  runs host tool diagnostics, generates Godot-first studio profiles, and drives
  a Pi-only staged wizard with fingerprint-bound prerequisite plans.
- **Native Pi MCP foundation:** `mcp-config` emits disabled Blender/Godot entries
  together from one explicit spec. Human-reported Pi health and a disposable
  Blender cube/export have bounded evidence; this is not an installed release
  or studio-role handoff. See the [MCP guide](docs/mcp-config.md).
- **Visual wizard:** plan-only preview with Bubble Tea UI (human visual
  acceptance: compact 80×25 PTY rendering, scrollable install plan, fingerprint
  disclosure). Installation (G7) runs only after explicit approval; the preview
  is read-only.
- **Core-game briefs:** `internal/workflows/coregame` and the `brief` CLI build
  a pending design draft from a structured request and validate it against a
  separately supplied recorded human decision. Validation checks consistency
  only and does not authenticate a human or grant execution authority.
- **Agent roles:** Two read-only specialists under `.pi/agents/`:
  - **Design specialist** — invoked once for a creation proposal; the resulting
    brief received human approval and passed CLI consistency validation
  - **Godot specialist** — invoked once for read-only 3D planning; returned
    bounded architecture and readiness steps without executing commands
- **Pi companion skills:**
  - `ogs-core` — classifies requests (creation/direct-phase/change/repair),
    activates needed responsibilities, returns bounded handoffs
  - `ogs-godot-change` — scopes one bounded mechanic change or repair in an
    existing trusted Godot 4 project
- **Test fixtures:**
  - 2D Godot scene with movement/collision (human playtest: window, cyan square,
    arrow keys, stopping on release)
  - 3D prototype "One Small Reach" in progress (Godot 4.7.2 version verified;
    scene not yet created)

**Not done yet:**

- Full installation (G7: skills loading, Engram persistence, resource validation)
- Godot 3D scenes (in progress: engine verified, fixture architecture planned)
- Blender production (V1-05)
- Audio integration (V1-07)
- Independent game QA (V1-08)
- Quest 3S + OpenXR/hands (V1-09)
- Full end-to-end journey with recovery (V1-11)

**Six studio responsibilities** are staged as on-demand capabilities, not
always-running agents. Two (design, Godot) have bounded exercise with documented
handoffs; four (art direction, Blender, audio, QA) remain unexercised as
studio responsibilities. The Blender MCP execution foundation is separate from
production-role exercise and artistic approval.

See [docs/project-status.md](docs/project-status.md) for the full capability and
evidence matrix.

## Goals

Build a studio that works end to end — limited in depth but not missing an
essential production area — while keeping humans in control:

- **Four entry points:** creation, direct-phase, change, repair (no forced
  concept interview every time)
- **Six responsibilities:** design, art direction, Blender production, Godot
  implementation, audio integration, independent game QA (on-demand, not
  always-running)
- **Godot-first:** explicit engine handoffs, bounded changes, verifiable outputs
- **Mandatory approval gates:** human reviews every handoff; no autonomous
  execution
- **Truthful provenance:** asset sources, model usage, and limits are recorded
- **Reuse host coordination:** Pi orchestration, Gentle Shell workflows,
  existing auth/models (no second orchestrator or provider system)

## What we learned from (adoption decisions)

OGS **reuses, adapts, and builds** on existing tools rather than reinventing
everything:

### From Gentle AI and Gentle Shell

**REUSE:**
- Pi as the host orchestrator (no competing coordinator)
- Gentle Shell's ODD/TDD/RDD workflows and guards
- Engram for hybrid persistence (Pi memory + studio state)
- Agent routing, subagent delegation, and work-unit commits
- Installer/profile model and prerequisite fingerprinting
- Native review integration (RDD) for bounded approval gates

**ADAPT:**
- Six studio responsibilities replace generic dev roles
- Game-specific approval gates (GDD slices, asset specs, QA)
- Godot-first handoffs (not engine-agnostic)
- Bounded entry points (creation/phase/change/repair)

**Source references:**
- [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) — installer,
  profiles, routing, hybrid persistence, RDD
- [gentle-shell](https://github.com/Gentleman-Programming/gentle-shell) — ODD,
  TDD, workflow chains, orchestrator patterns

### From Claude Code Game Studios

**ADAPT:**
- Studio hierarchy concept (roles working as a team, not isolated agents)
- Game-dev orchestration (idea → concept → GDD → implementation → QA)
- Phase boundaries and handoff discipline
- The insight that AI should work like a real studio, not a prompt generator

**BUILD (OGS-specific):**
- Go CLI for workspace bootstrap and diagnostics
- Core-game brief library with draft/validation separation
- Godot 4 bounded change skill
- Visual wizard with fingerprint approval
- Explicit provenance tracking for assets and decisions

**Source reference:**
- [Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios) —
  studio hierarchy, game-dev flow, team-like coordination

### What OGS does NOT reuse

- CCGS's full `.claude/` prompt catalog (adapted the studio flow, not the prompts)
- Gentle AI's SDD (Spec-Driven Development) workflow (OGS uses direct handoffs)
- Any authentication, model config, or PATH from upstream (users supply their own)

**Adoption details:** See [docs/ogs-adoption-matrix.md](docs/ogs-adoption-matrix.md)
for the full reuse/adapt/build matrix with evidence levels.

## Getting oriented

| Document | What it answers |
|---|---|
| [docs/project-status.md](docs/project-status.md) | What works today, and at what evidence level. |
| [docs/roadmap.md](docs/roadmap.md) | The 11 ordered V1 tasks, mandatory milestones, and deferred breadth. |
| [docs/ogs-adoption-matrix.md](docs/ogs-adoption-matrix.md) | What we reuse, adapt, or build; evidence and priority. |
| [docs/core-game-briefs.md](docs/core-game-briefs.md) | The core-game brief library and `brief` CLI in detail. |
| [docs/mcp-config.md](docs/mcp-config.md) | Native Pi MCP configuration, prerequisites, evidence, and trust limits. |
| [docs/generated-artifacts.md](docs/generated-artifacts.md) | Generated artifact classification and policy. |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute. |
| [AGENTS.md](AGENTS.md) | Portable agent instructions for this repository. |

## Repository layout

| Path | Contents |
|---|---|
| `cmd/game-studio/` | CLI entry point. |
| `internal/` | Installer, wizard, commands, contracts, persistence, and tooling. |
| `skills/` | Canonical Pi companion skills and their Go embedding. |
| `.pi/agents/` | Two read-only project-local agent roles. |
| `tests/`, `testdata/` | Node checkers, fixtures, and Godot scenes. |
| `docs/` | Canonical documentation. |

## Building and testing the source

Go 1.26.2 is required, and Linux/WSL is the first supported platform.

```sh
go build ./cmd/game-studio
go test ./... -count=1
```

**GitHub Actions** runs clean builds on `ubuntu-latest` with Go 1.26.2 for all
PRs. Local testing has additional cautions:

- The repository does not vendor Go modules. On a cold module cache, `go test`
  downloads dependencies from the configured module proxy.
- Tests are **not fully isolated**. Workspace resolution walks ancestor
  directories, so running inside a checkout with an ancestor workspace marker
  or `openspec/config.yaml` can cause wizard tests to write through that
  ancestor. Run in an isolated copy with no ancestor workspace.
- `internal/cli/commands` tests install a no-op `engram` shim for safety; this
  is not a setup instruction.

Optional Node checks exercise the Godot fixture. The first command runs
structural checks only; the second needs an explicit engine binary:

```sh
GODOT_BIN='' node --test tests/pi-godot-change.test.mjs
GODOT_BIN=/absolute/path/to/godot4 node --test tests/pi-godot-change.test.mjs
```

Setting `GODOT_BIN=''` prevents inherited values. These checks cover movement
behavior only, not visual quality, fun, exports, or full-studio readiness.

## CLI commands

| Command | Behavior |
|---|---|
| `game-studio init` | Bootstraps and verifies the workspace skeleton. |
| `game-studio env-check` | Runs host tool diagnostics for Godot, Blender, or selected tools. |
| `game-studio generate` | Emits flat `studio-profile.<engine>.md` / `.summary.md`, pack metadata, and hybrid map. |
| `game-studio wizard` | Runs the Pi-only staged installer/personalization wizard. |
| `game-studio smoke` / `smoke-suite` | Validates generated layout and content. |
| `game-studio brief draft` / `brief check` | Builds a pending core-game draft, or checks one against a recorded decision. |
| `game-studio mcp-config` | Emits a create-only `WORKSPACE/.pi/mcp.json` for explicit local Blender/Godot MCP runtimes from a JSON spec. |

**Wizard modes** (mutually exclusive):
- `--plan-only` — read-only preview with fingerprint (no writes)
- `--metadata-only` — writes profile/workspace/artifacts, attempts Engram
  (not read-only; can return partial error if Engram unavailable)
- `--approve-plan <fingerprint>` + `--non-interactive` — real installation

`--non-interactive` alone never authorizes installation. See
[docs/core-game-briefs.md](docs/core-game-briefs.md) for details.

## Pi companion skills

Load skills explicitly without changing personal settings:

```sh
pi --skill ./skills/ogs-core/SKILL.md --skill ./skills/ogs-godot-change/SKILL.md
```

Then invoke `/skill:ogs-core` or `/skill:ogs-godot-change`. Loading grants no
install/download/destructive authority; those need concrete plans and explicit
human consent.

**Agent roles** under `.pi/agents/` are read-only definitions:
- **Design specialist** — invoked once; returned 8-section creation proposal
  that received human approval and passed CLI consistency validation
- **Godot specialist** — invoked once for 3D readiness planning; no commands
  executed

See [docs/project-status.md](docs/project-status.md) for bounded acceptance and
remaining work. Source-checkout paths are not installed-pack evidence.

## Licensing

First-party source is MIT-licensed; see [LICENSE](LICENSE) (Copyright (c) 2026
IrrealV).

Third-party components keep their own licenses. This repository's MIT license
does **not** relicense dependencies or assets. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for recorded attributions.

## Credits

OGS learns from and builds on:

- **[gentle-ai](https://github.com/Gentleman-Programming/gentle-ai)** —
  installer/profile model, workflow routing, hybrid persistence (Engram),
  native review integration (RDD), and the "one tool, many workflows" approach.
  OGS reuses Pi orchestration and adapts bounded approval gates for game-dev.
  
- **[gentle-shell](https://github.com/Gentleman-Programming/gentle-shell)** —
  ODD (Organic Driven Development), TDD, workflow chains, and orchestrator
  patterns. OGS reuses these disciplines and adapts them to six studio
  responsibilities.
  
- **[Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios)** —
  studio hierarchy, game-dev orchestration (idea → GDD → implementation → QA),
  and the insight that AI should work like a real team. OGS adapts the studio
  flow without copying prompts or catalog structure.

OGS is **not** a fork or direct copy of any upstream project. It reuses
coordination, adapts workflows, and builds game-specific capabilities on top.

See [docs/ogs-adoption-matrix.md](docs/ogs-adoption-matrix.md) for the full
reuse/adapt/build breakdown.

<a href="https://github.com/Gentleman-Programming/gentle-ai">
  <img width="220" src="https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/docs/assets/brand/built-with-gentle-ai.png" alt="Built with Gentle-AI" />
</a>

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution flow, and
[AGENTS.md](AGENTS.md) if you are an agent working in this repository.
