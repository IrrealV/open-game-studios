---
name: ogs-godot-change
description: "Trigger: Godot mechanic change, Godot gameplay repair. Safely scope, implement, and verify one mechanic in a trusted Godot 4 project."
license: MIT
metadata:
  author: open-game-studios
  version: "1.0"
---

## Activation Contract

Use this skill for an approved, bounded mechanic change or repair in an existing trusted Godot 4 project. Require Pi and gentle-shell to be installed separately; this package does not install or bundle them.

## Hard Rules

- Preserve user changes. Never replace project instructions, personas, models, profiles, `AGENTS.md`, or `SYSTEM` files.
- Treat conversation or tool-response approval as authoritative. Creative approval never authorizes installs, downloads, destructive actions, privilege escalation, or global configuration changes.
- State the mechanic, expected behavior, relevant scene/scripts, allowed edit scope, and checks before writing. Resolve material ambiguity with one focused question.
- Follow [Godot setup and consent](references/godot-setup.md) when Godot 4 readiness is unknown. Stop at its approval gate.
- Keep edits and delegated work inside the approved scope. Use the existing harness's bounded worker and verifier roles when available; do not imply that planning text is runtime enforcement.

## Decision Gates

| Observation | Action |
|---|---|
| Project trust or mechanic intent is unclear | Stop and request the missing approval or decision. |
| Godot 4 is absent, wrong, or unusable | Follow the local setup reference; do not run gameplay. |
| Scope is clear and runtime is ready | Implement the smallest coherent change and focused tests. |
| Required checks fail | Report evidence; do not claim completion or broaden scope. |

## Execution Steps

1. Read project instructions and the complete affected scene, scripts, tests, and mechanic context.
2. Inspect working-tree state and name the exact edit and verification scopes.
3. Confirm Godot 4 readiness without mutating the host or project.
4. Implement one bounded behavior change while preserving unrelated work.
5. Run project-native focused checks, then approved Godot headless checks on a temporary project copy when isolation is required.
6. Report measured behavior, command results, changed paths, skipped checks, and remaining visual or gameplay judgment.

## Output Contract

Return the approved mechanic outcome, exact changed paths, exact checks and observed results, preserved state, and known limits. Describe headless results only as evidence for the selected behavior; they do not prove fun, visual quality, broad security isolation, or full-game correctness.

## References

- [Godot setup and consent](references/godot-setup.md)
