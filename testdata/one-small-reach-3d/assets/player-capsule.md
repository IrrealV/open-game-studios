---
version: 1
asset: player-capsule
art_bible: ../art-bible.md
approval_state: pending
---

# Asset Spec — Player Capsule

Metadata-only specification for the player representation in the disposable
`One Small Reach` 3D fixture. No asset is generated, imported, or mutated here.

## Intent

Represent the player character as a single, instantly readable capsule that
matches the movement controller.

## Gameplay Use

The capsule is the moving body that the player steers, and it is the reference
point for the collection interaction with the collectible sphere.

## Narrative Use

None — this is a non-narrative, disposable test fixture.

## Art Bible Links

- Clean minimalism: one primitive, no extra detail.
- Distinct silhouettes: a tall capsule never reads as the round collectible.
- Palette: player cyan (#00B3FF).

## Output Targets

- Godot `CapsuleMesh` engine primitive (radius 0.3, height 1.6).
- Cyan `StandardMaterial3D` using #00B3FF.
- Baseline: the current fixture uses an engine primitive with different
  dimensions and a close cyan; this spec is the direction target for a future
  Blender-authored upgrade, not a change to the fixture.

## Prompt Metadata

- "simple cyan capsule, minimalist 3D game character"
- Negative prompt: "no textures, no character detail, no realistic materials"

## Import Hints

- If upgrading to a custom mesh: keep vertical orientation and a centered pivot.
- Declare meters as the scale unit and match the capsule to the controller's
  feet-level origin.
- Import metadata only; no Godot or Blender file is mutated by this spec.

## Acceptance Criteria

- Player color is cyan #00B3FF.
- Capsule is visible from the fixed orthographic camera.
- Capsule silhouette and color are distinct from the collectible sphere #FFD700.
