# Game-Studio — project context

This is a short, accurate compatibility entry point for **Game-Studio** / Open
Game Studios (OGS), for tools or people that previously consumed a long
single-page context document. The canonical, maintained status no longer lives
here; it lives in the pages below.

## One-sentence summary

Open Game Studios is a human-led, Godot-first game-production workspace for the
Pi host and the Gentle Shell coordinator, and Game-Studio is the studio profile
it installs and generates.

## Core principle

OGS helps a person create a **good** game through disciplined design, visual
direction, production workflows, and review gates. It is not a one-prompt
generic game generator: the AI is a tool and the human remains the director.

## Canonical documents

| Question | Document |
|---|---|
| Current capability and evidence status | [docs/project-status.md](docs/project-status.md) |
| Source module map and boundaries | [docs/architecture.md](docs/architecture.md) |
| Ordered work and deferred breadth | [docs/roadmap.md](docs/roadmap.md) |
| Core-game brief library and CLI | [docs/core-game-briefs.md](docs/core-game-briefs.md) |
| Generated artifact policy | [docs/generated-artifacts.md](docs/generated-artifacts.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Agent instructions | [AGENTS.md](AGENTS.md) |
| Repository entry point | [README.md](README.md) |

## Current shape

- Go CLI (module `open-game-studios`, Go 1.26.2, Linux/WSL first) with `init`,
  `env-check`, `generate`, `wizard`, `smoke`, `smoke-suite`, and `brief`.
- A Pi-only staged installer/wizard with a fingerprint-bound approval gate. The
  plan-only preview is read-only; real installation is separate.
- Two Pi companion skills (`ogs-core`, `ogs-godot-change`) and two read-only
  project-local roles (`.pi/agents/`).
- A source-accepted core-game brief library and CLI that produce pending drafts
  and validate consistency against a separately supplied recorded decision. That
  validation does not authenticate a human or grant authority.
- Godot as the active engine pack; Unity and UE5 are deferred.

## What this is not (yet)

Not a finished v1, not an installed product, and not a released distribution.
Godot 3D, Blender production, audio integration, independent game QA, the Quest
3S device route, and the full end-to-end journey remain open. See the roadmap.
