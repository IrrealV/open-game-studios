---
version: 1
asset: floor-plane
art_bible: ../art-bible.md
approval_state: pending
---

# Asset Spec — Floor Plane

Metadata-only specification for the play surface in the disposable
`One Small Reach` 3D fixture. No asset is generated, imported, or mutated here.

## Intent

Provide a flat, neutral surface that anchors the scene and makes movement
bounds visually readable.

## Gameplay Use

The floor defines the visible movement bounds and gives the player a stable
reference for horizontal distance and framing.

## Narrative Use

None — this is a non-narrative, disposable test fixture.

## Art Bible Links

- Clean minimalism: a single flat plane, no detail or texture.
- Palette: floor neutral gray (#4D4D4D) against the dark blue background (#26293B).

## Output Targets

- Godot `PlaneMesh` engine primitive (8 × 8).
- Gray `StandardMaterial3D` using #4D4D4D.
- Baseline: the current fixture uses an engine primitive at 8 × 8 with a darker
  blue-gray; this spec is the direction target for a future upgrade, not a
  change to the fixture.

## Prompt Metadata

- "simple gray floor plane, minimal 3D scene"
- Negative prompt: "no textures, no grid details, no realistic materials"

## Import Hints

- If upgrading to a custom mesh: keep the plane flat and horizontal.
- Keep a centered origin so bounds and camera framing stay predictable.
- Import metadata only; no Godot or Blender file is mutated by this spec.

## Acceptance Criteria

- Floor color is gray #4D4D4D.
- Floor size is 8 × 8.
- Surface is flat and horizontal at Y = 0.
