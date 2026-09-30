# Proposal: First Implementation Batch

## Problem
`open-game-studios` needs a usable first slice of `Game-Studio` without collapsing CCGS knowledge or overbuilding engine support.

## Goals
- Bootstrap an OpenCode-only `Game-Studio` profile generator.
- Preserve CCGS semantics with layered packaging.
- Start with Godot, keep Unity and UE5 as future packs.
- Use hybrid persistence: OpenSpec + Engram.

## Non-goals
- No real MCP runtime yet.
- No full Unity/UE5 implementation yet.
- No aggressive reduction of CCGS knowledge.

## Approach
- Build a single profile generator.
- Use a layered model: core profile + engine packs + CCGS mapping contract.
- Emit concrete OpenCode artifacts for Godot first.
- Keep persistence thin and additive.

## Risks
- Scope creep across engines.
- Traceability gaps if OpenSpec artifacts are missing.
- No real automated tests yet.

## Next
Proceed to design and task execution for the first implementation batch.
