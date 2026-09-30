# Game-Studio working document

This file is the repository's root working document for the **Game-Studio**
profile. The canonical, current project documentation now lives in the pages
below; this page is a short compatibility entry point so older references keep
working without duplicating status inventories.

## What Game-Studio is

Game-Studio is the single Pi-targeted studio profile that OGS installs and
generates. It combines multi-agent studio semantics with phase-driven
execution and explicit human review gates, and it starts with Godot.

## Canonical documents

| Question | Document |
|---|---|
| What works today, and at what evidence level? | [docs/project-status.md](docs/project-status.md) |
| How is the code organized and what are the boundaries? | [docs/architecture.md](docs/architecture.md) |
| What is ordered next, and what is deferred? | [docs/roadmap.md](docs/roadmap.md) |
| How do I contribute? | [CONTRIBUTING.md](CONTRIBUTING.md) |
| How should an agent work here? | [AGENTS.md](AGENTS.md) |
| What does the project's entry point claim? | [README.md](README.md) |

## Non-negotiables

- Pi is the only current target; Pi owns authentication and model selection, and
  Gentle Shell owns orchestration.
- Persistence is hybrid: OpenSpec for change artifacts and Engram for memory.
- Human approval gates are mandatory; creative and scope decisions are never
  auto-approved.
- Godot is the first engine pack; Unity and Unreal Engine 5 are deferred.
