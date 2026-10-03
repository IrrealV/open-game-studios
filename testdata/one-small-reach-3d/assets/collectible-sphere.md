---
version: 1
asset: collectible-sphere
art_bible: ../art-bible.md
approval_state: pending
---

# Asset Spec — Collectible Sphere

Metadata-only specification for the collectible in the disposable
`One Small Reach` 3D fixture. No asset is generated, imported, or mutated here.

## Intent

Represent the single collectible as a compact, high-contrast sphere that reads
immediately against the floor and background.

## Gameplay Use

The sphere is collected once when the player is within the collection distance,
its visible state changes, and gameplay continues afterward.

## Narrative Use

None — this is a non-narrative, disposable test fixture.

## Art Bible Links

- Clean minimalism: one primitive, no extra detail.
- Readable from above: a round, high-contrast shape stays legible from the fixed camera.
- Palette: collectible gold (#FFD700).

## Output Targets

- Godot `SphereMesh` engine primitive (radius 0.3).
- Gold `StandardMaterial3D` using #FFD700.
- Baseline: the current fixture uses an engine primitive with a different radius
  and a close gold; this spec is the direction target for a future upgrade, not
  a change to the fixture.

## Prompt Metadata

- "simple golden sphere, minimalist collectible"
- Negative prompt: "no textures, no glow effects, no realistic materials"

## Import Hints

- If upgrading to a custom mesh: keep a centered pivot.
- Match the sphere's Y position to the player height so it reads at eye level.
- Import metadata only; no Godot or Blender file is mutated by this spec.

## Acceptance Criteria

- Collectible color is gold #FFD700.
- Collection triggers at horizontal distance ≤ 0.6.
- A visible state change occurs on collection.
