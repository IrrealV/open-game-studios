# Open Game Studios

Open Game Studios (OGS) is a human-led, Godot-first game-production workspace
for the Pi host and the Gentle Shell coordinator. Today this repository holds
three honest pieces: a Go CLI installer/generator, a small Pi companion skill
package, and a source-accepted core-game brief library and CLI.

OGS is **not** a finished v1, not an installed product, and not a one-prompt
game generator. The first-party source is MIT-licensed (see
[Licensing](#licensing)).

Canonical project home (planned): <https://github.com/IrrealV/open-game-studios>.
This page does not assert that the repository is already public, that it is
published, or that any installable release exists.

## Current reality

- **Installer/generator (Go):** the `game-studio` CLI bootstraps a workspace,
  runs host tool diagnostics, generates a Godot-first studio profile, and drives
  a Pi-only staged wizard.
- **Wizard:** prepares a fingerprint-bound prerequisite plan. A plan-only
  preview is read-only; installation only runs after an explicit approval of
  the exact printed fingerprint. The preview has human visual acceptance.
- **Pi companion skills:** `ogs-core` routes a request and returns a bounded
  handoff; `ogs-godot-change` scopes one bounded mechanic change or repair in a
  trusted Godot 4 project.
- **Core-game briefs:** `internal/workflows/coregame` and the `brief` CLI build
  a pending design draft and validate it against a separately supplied recorded
  decision. Validation is consistency-only and does not authenticate a human.
- **Not done yet:** real installation (G7), Godot 3D, Blender production, audio
  integration, independent game QA, the Quest 3S route, and the full end-to-end
  journey. The six studio responsibilities are staged, not operational agents.

See [docs/project-status.md](docs/project-status.md) for the full capability and
evidence matrix.

## Goals

Make a studio that works end to end — limited in depth but not missing an
essential production area — while keeping the human in control:

- creation, direct-phase, change, and repair entry points without forcing a
  full concept interview every time;
- six on-demand responsibilities (design, art direction, Blender production,
  Godot implementation, audio, independent game QA);
- Godot-first execution with explicit engine handoffs;
- mandatory human approval gates and truthful provenance;
- reuse of the host's coordination, authentication, and models — no second
  orchestrator or provider system.

## Getting oriented

| Document | What it answers |
|---|---|
| [docs/project-status.md](docs/project-status.md) | What works today, and at what evidence level. |
| [docs/architecture.md](docs/architecture.md) | Source module map, boundaries, and the draft-vs-decision trust model. |
| [docs/roadmap.md](docs/roadmap.md) | The 11 ordered V1 tasks, mandatory milestones, and deferred breadth. |
| [docs/core-game-briefs.md](docs/core-game-briefs.md) | The core-game brief library and `brief` CLI in detail. |
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
| `tests/`, `testdata/` | Node checkers, fixtures, and the Godot fixture. |
| `docs/` | Canonical documentation. |

## Building and testing the source

Go 1.26.2 is required, and Linux/WSL is the first supported platform.

```sh
go build ./cmd/game-studio
go test ./... -count=1
```

Honest cautions before you run this:

- The repository does not vendor Go modules. On a cold module cache, `go test`
  downloads this module's dependencies from the configured module proxy.
- The tests do not run engines or download game assets, but they are **not fully
  isolated from their surroundings**. Workspace resolution walks ancestor
  directories, so running the suite inside a checkout that has an ancestor
  workspace marker or an ancestor `openspec/config.yaml` can cause some wizard
  tests to write managed configuration through that ancestor instead of their
  own temporary directory. Run the suite in an isolated copy of the checkout
  with no ancestor workspace, and review anything a run wrote. Clean-clone
  execution has not been observed for this documentation slice.
- The `internal/cli/commands` tests install a no-op `engram` shim so tests never
  touch a personal memory database. That shim exists for test safety only; it is
  not a setup or installation instruction.

Optional Node checks exercise the Godot fixture. The first command runs the
structural checks only; the second is an opt-in native Godot headless example
that needs an explicit engine binary and was **not** run for this slice:

```sh
GODOT_BIN='' node --test tests/pi-godot-change.test.mjs
GODOT_BIN=/absolute/path/to/godot4 node --test tests/pi-godot-change.test.mjs
```

The source reads `GODOT_BIN` from the environment only and never auto-detects an
engine, so setting `GODOT_BIN=''` explicitly prevents an inherited value from
launching Godot by accident. These checks cover the selected movement behavior
only. They do not prove visual quality, fun, exports, or full-studio readiness.

## CLI commands

| Command | Behavior |
|---|---|
| `game-studio init` | Bootstraps and verifies the workspace skeleton. |
| `game-studio env-check` | Runs host tool diagnostics for Godot, Blender, or selected tools. |
| `game-studio generate` | Emits flat `studio-profile.<engine>.md` / `.summary.md`, pack metadata, and the hybrid map. |
| `game-studio wizard` | Runs the Pi-only staged installer/personalization wizard. |
| `game-studio smoke` / `smoke-suite` | Validates generated layout and content. |
| `game-studio brief draft` / `brief check` | Builds a pending core-game draft, or checks one against a recorded decision. |

The wizard exposes two **mutually exclusive** non-installing modes, and combining
them is rejected: `--plan-only` renders the read-only prerequisite plan and
fingerprint without writing, while `--metadata-only` runs the artifact-only path.
`--metadata-only` writes local profile/pack artifacts, the workspace config, and
the final artifact, and attempts the Engram memory write-through, so it is **not**
read-only and a missing Engram companion can return a partial error. Real
prerequisite installation runs only after an explicit
`--approve-plan <fingerprint>` approval, which itself requires
`--non-interactive`; `--non-interactive` alone never authorizes installation.
Details are in [docs/architecture.md](docs/architecture.md) and
[docs/core-game-briefs.md](docs/core-game-briefs.md).

## Pi companion skills

`skills/ogs-core` is the entry skill: it classifies a request as creation, a
direct phase, a change, or a repair, activates only the responsibilities it
needs, and returns a bounded handoff using its local
[handoff contract](skills/ogs-core/references/handoff-contract.md).
`skills/ogs-godot-change` is the bounded skill for changing or repairing one
mechanic in an existing trusted Godot 4 project.

Load the skills you intend to use for one session without changing personal
settings. `--skill` is repeatable, and loading `ogs-core` does not register the
sibling skill, so name each one explicitly:

```sh
pi --skill ./skills/ogs-core/SKILL.md --skill ./skills/ogs-godot-change/SKILL.md
```

Then invoke `/skill:ogs-core`, or `/skill:ogs-godot-change` for a bounded Godot
change. Loading a skill grants no install, download, or destructive authority;
those still require a concrete plan and explicit human consent. When Godot 4
readiness is unknown, [the Godot setup reference](skills/ogs-godot-change/references/godot-setup.md)
describes an optional, reviewable setup plan and stops at its approval gate.

The two files under `.pi/agents/` are read-only project role definitions. The
[Godot role](.pi/agents/ogs-godot-specialist.md) has been host-discovered and
used once for a bounded source-local handoff. The
[design role](.pi/agents/ogs-design-specialist.md) exists and is host-listable
but has not been invoked or independently accepted. Source-checkout paths are
not installed-pack evidence.

## Licensing

First-party source in this repository is licensed under the MIT License; see
[LICENSE](LICENSE) (Copyright (c) 2026 IrrealV).

Third-party components, dependencies, and assets keep their own licenses and
notices. This repository's MIT license does **not** relicense any dependency,
upstream project, or asset. A full third-party attribution audit is still
pending; no external notices are fabricated here, and existing notices are not
removed. Recorded third-party attribution is listed in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Credits

This project learns from:

- **[Claude Code Game Studios](https://github.com/Donchitos/Claude-Code-Game-Studios)** —
  inspiration for studio hierarchy, game-dev orchestration, and the idea that an
  AI should work like a real team.
- **[gentle-ai](https://github.com/Gentleman-Programming/gentle-ai)** —
  inspiration for the installer/profile model, workflow routing, hybrid
  persistence, and the "one tool, many workflows" approach.

Game-Studio is **not** a direct copy of either project.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution flow, and
[AGENTS.md](AGENTS.md) if you are an agent working in this repository.
