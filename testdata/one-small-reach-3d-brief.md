# One Small Reach — 3D Prototype Brief

**Status:** Human-approved disposable fixture for V1-03. Not the production game.

## Exact Mechanics

### Scene Setup
- **Floor:** 8×8 units (Godot MeshInstance3D with PlaneMesh)
- **Player:** Capsule at start position X/Z=(-2, 0), Y at feet level
- **Collectible:** One sphere at X/Z=(2, 0), Y at player height
- **Camera:** Fixed orthographic, positioned to show full scene

### Movement (WASD + Arrow Keys)
- **Speed:** 3 units/second
- **Diagonal movement:** Normalized (no speed boost on diagonals)
- **Stop behavior:** Immediate stop on key release
- **Bounds:** Clamp X/Z to ±3.5 (keep player on floor)
- **Y movement:** None (no jump)

### Collection
- **Trigger distance:** Horizontal distance ≤ 0.6 units
- **Collect once:** Only first trigger counts
- **On collection:**
  - Hide/remove sphere
  - Display one completion message (Label or similar)
  - Movement continues normally afterward

## Out of Scope

- Jump, enemies, inventory, score, timer
- Progression, restart functionality
- Custom assets, textures, models
- Audio, music, sound effects
- Export builds, controller support
- VR, Quest, device-specific features

## Technical Constraints

- **Engine:** Godot 4.x
- **Materials:** Engine primitives only (no custom assets)
- **Reference:** 2D fixture at `testdata/godot-minimal-2d/` (unchanged, read-only reference)
- **Target:** `testdata/one-small-reach-3d/`
- **Advisory line count:** ~400 authored lines (not a hard limit)

## Files to Create

- `project.godot` — Godot 4 project configuration
- `main.tscn` — Main scene (Node3D root)
- `main.gd` — Scene script (movement + collection logic)
- `verify_mechanic.gd` — GDScript test for deterministic verification
- `README.md` — Concise fixture description (optional)

## Acceptance Criteria

1. Scene loads without errors
2. Player starts at correct position (-2, 0)
3. WASD and arrow keys produce movement
4. Speed matches 3 units/sec
5. Diagonal movement is normalized
6. Key release stops movement immediately
7. Player stays within ±3.5 bounds
8. Collection triggers at distance ≤ 0.6
9. Sphere hides and message shows on collection
10. Movement continues after collection
11. Second collection attempt has no effect

## Verification Approach

**Test-first recommended:** Following the 2D fixture pattern, write `verify_mechanic.gd` with deterministic checks using `physics_step` before implementing `main.gd`. This allows RED → GREEN → refactor workflow.

If deterministic headless testing proves impractical, provide rationale and proceed with direct implementation + manual verification checklist.

## Source

Human-approved design gate, consistency-checked, recorded in V1-02 delivery (c6ad297).
