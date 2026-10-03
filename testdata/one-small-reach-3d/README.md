# One Small Reach — 3D Prototype

Disposable Godot 4 fixture for V1-03. Tests basic 3D movement, bounds, and collection mechanics.

## Run

```sh
godot --path testdata/one-small-reach-3d
```

## Verify (headless)

```sh
godot --headless --path testdata/one-small-reach-3d --script verify_mechanic.gd
```

Optional speed check:
```sh
godot --headless --path testdata/one-small-reach-3d --script verify_mechanic.gd -- --expected-speed=3.0
```

## Controls

- **WASD** or **Arrow Keys** — Move player
- No jump, no other actions

## Constants

| Constant | Value | Description |
|---|---|---|
| SPEED | 3.0 | Movement speed (units/sec) |
| BOUNDS | 3.5 | X/Z clamp (±3.5) |
| COLLECTION_RADIUS | 0.6 | Horizontal distance threshold |

## Files

- `project.godot` — Input map (WASD + arrows), display config
- `main.tscn` — Scene tree: floor 8×8, player at (-2,0), collectible at (2,0), orthographic camera
- `main.gd` — Movement logic, bounds clamping, collection trigger
- `verify_mechanic.gd` — Headless verifier (12 checks: input map, positions, movement, normalized diagonal, stop, bounds, collection threshold bracket, once-only, movement-after)

## Out of Scope

- Jump, enemies, inventory, score, timer, progression, restart
- Custom assets, audio, export
- Controller, VR, device-specific features

## Acceptance

Verified headless with `verify_mechanic.gd` passing all checks. GUI playtest required for visual framing confirmation.
