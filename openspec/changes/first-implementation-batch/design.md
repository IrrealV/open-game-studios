# Design: First Implementation Batch

## System Boundary
- **Core**: `Game-Studio` profile orchestration, CLI, templates, smoke checks.
- **Packs**: engine-specific metadata and output layouts.
- **Mapping**: CCGS abstract patterns translated into engine recipes.

## Artifact Model
- `GAME-STUDIO.md` as root working doc.
- `.game-studio/workspace.manifest.json` as workspace seed.
- `profiles/game-studio/*` as generated profile assets.
- Godot pack artifacts: profile markdown, hybrid-map JSON, pack config JSON, summary markdown.

## Persistence
- **OpenSpec**: structured change docs and task tracking.
- **Engram**: decisions, context, apply-progress, and semantic notes.

## MCP Strategy
- Baseline: Engram + Context7.
- Extra MCPs only if essential.

## Future Packs
- Unity and UE5 remain placeholders with the same pack contract.

## Validation
- Use smoke validation for layout and generated artifact content.
- No strict TDD yet because no test runner exists.
