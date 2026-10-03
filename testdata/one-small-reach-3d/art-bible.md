---
version: 1
approval_state: approved
approved_by: "IrrealV"
---

# Art Bible — One Small Reach (disposable 3D fixture)

Direction for the disposable `One Small Reach` 3D fixture. This document is
**pending human approval**. It is metadata-only: it describes intended visual
direction and does not generate, import, or mutate any asset.

## Visual Pillars

- Clean minimalism: every element earns its place; nothing decorative.
- Readable from above: shapes and colors stay legible under the fixed orthographic camera.
- Distinct silhouettes: player, collectible, and floor are never confusable in shape or color.

## Style

Simple geometric engine primitives only, with flat solid colors, no textures, no
complex materials, and no baked lighting detail. The look is a legible
greybox-to-final test scene, not a fully produced art style.

## Palette

- Player: cyan (#00B3FF)
- Collectible: gold (#FFD700)
- Floor: neutral gray (#4D4D4D)
- Background: dark blue (#26293B)

## References

- Godot primitive meshes (`CapsuleMesh`, `SphereMesh`, `PlaneMesh`) as the baseline geometry language.
- Minimal 3D test scenes and greybox blockouts used for readability checks.
- `testdata/one-small-reach-3d/main.tscn` as the current disposable baseline, not as an approved final look.

## Constraints

- Engine primitives only; no custom models in this fixture.
- No textures, no imported materials, no UV work.
- Fixed orthographic camera; direction must read from the default framing.
- Visual scope only — no audio, music, or SFX.
- The disposable fixture budget is roughly 400 authored lines of engine source.

## Negative Guidance

- No realistic textures or photoreal materials.
- No complex shaders or material graphs.
- No character detail, rigging, or facial features.
- No particle effects, post-processing, or cinematic lighting.
- No color outside the declared palette.
