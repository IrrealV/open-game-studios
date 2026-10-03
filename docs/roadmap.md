# Roadmap

This roadmap orders the work for a **usable OGS v1**: a human-led, Godot-first
studio that works end to end, limited in depth but not missing an essential
production area. It uses portable task IDs instead of private issue links. No
public issue is created automatically from this file.

Status reflects the checkpoint in [project-status.md](project-status.md).
"Accepted" always means accepted at the stated evidence level only.

## V1 tasks

| ID | Task | Status | Acceptance boundary |
|---|---|---|---|
| V1-01 | Load the OGS core and demonstrate a domain handoff | **Done** (source-local technical scope) | Core guidance loaded, grouped skill delivery, a discovered Godot role, and one real bounded handoff with a returned result. |
| V1-02 | Make design and its existing contracts executable | **Done** (bounded source-local creation journey) | Turn a request into a bounded approved work brief through the design role, supporting creation/direct-phase/change/repair. The design role was invoked once for a creation request and returned an eight-section `gdd-slice` creation proposal (a `coregame.DraftRequest`), not an approval or executable plan; the CLI produced the canonical pending draft from that proposal and accepted the downstream consistency of an observed human approval of that exact brief (consistency only, not authentication or consent). Creation/direct-phase/change/repair remain supported by accepted source and fixtures; only the one creation journey was observed live. |
| V1-03 | Implement and check a small Godot 3D task | Open | Godot role creates or changes a disposable scene/script with bounded criteria and independent checking. Engine evidence plus a human-tryable result, not a generated profile. |
| V1-04 | Produce an actionable visual brief and asset specification | Open | Art direction uses the existing Art Bible / Asset Spec / Asset Audit contracts; distinguishes references, imported assets, and production operations. |
| V1-05 | Produce a real Blender asset and export | Open | One verified Blender execution route, basic modeling/material work, a real editable asset and export, and agreed scale/orientation/material requirements. A detected binary or named placeholder is not enough. |
| V1-06 | Integrate and validate the Blender asset in Godot | Open | Godot import/scene integration validating paths, scale, materials, and collisions, with an independent visual/technical audit. |
| V1-07 | Integrate real music, ambience, and effects | Open | Activate the audio role and a lean audio contract (intended use, source/license, file, duration/loop, checks); basic levels, loops, and transitions with actual playback plus human listening. |
| V1-08 | Exercise independent game QA and recovery | Open | Consolidate prior checks into a reproducible game/asset/audio QA route and deliberately exercise bounded failures and recovery. Game QA stays distinct from code/native review, artistic approval, and human playtest. |
| V1-09 | Validate the minimal Quest 3S route | Open | Validate Godot Android/OpenXR/export and hand-interaction tooling in the test project, with actual build/export and separately observed headset interaction. Desktop engine execution alone does not certify VR. |
| V1-10 | Deliver the working pack through the existing installer | Open | Connect the resources/tool requirements to the existing installation path and close the relevant G7 acceptance on an authorized Linux/WSL target. |
| V1-11 | Complete the full journey and demonstrate continuation | Open | Run specification → Blender → Godot → audio → independent QA → human playtest, and demonstrate persisted decisions with a later resumed task. |

V1 tasks proceed in sequence, one writer at a time. Each task must deliver
observable behavior with checks; a written role definition, a detected binary,
or metadata alone does not establish readiness.

## Mandatory milestones (not optional)

- **Blender production**: one real editable asset and export (V1-05) integrated
  into Godot (V1-06).
- **Art direction**: an approved, traceable visual brief and asset spec (V1-04).
- **Audio**: real music/SFX with loops, levels, and transitions (V1-07).
- **Independent game QA**: an executable QA/recovery route (V1-08) that is not a
  self-approved author report.
- **Quest 3S**: the separately gated Android/OpenXR/hand-interaction route
  (V1-09).

## Deferred breadth (post-v1, important but not blocking)

| Area | Deferred work |
|---|---|
| Image providers | Provider-native image generation, ComfyUI adapter, and visual source/production lane separation. |
| Further engines | Unity pack and Unreal Engine 5 pack after the Godot-first flow matures. |
| Platforms | Clean Windows and macOS validation; Linux/WSL is first. |
| Audio generation | Generative music provider selection (audio *integration* is required; a specific generator is not). |
| Collaboration | Multi-person workflow management beyond the single-person first cut. |
| Assets | Advanced / AAA asset generation and wide asset pipelines. |
| Release | A public-release gate that validates clean install and representative workflows from zero required tools on dedicated platforms, deliberately provoking errors. |

## Release-readiness gate

A public release is blocked until OGS passes clean, zero-tool install plus
representative real game workflows on the supported platforms, and until
missing tools, PATH, providers, engine, permissions, and workflow failures
produce clear, actionable, non-destructive diagnostics. A setup that only works
on a prepared developer machine does not count. This gate is separate from
per-task acceptance.

## How to pick up work

Read [project-status.md](project-status.md) first, then [CONTRIBUTING.md](../CONTRIBUTING.md)
and [AGENTS.md](../AGENTS.md). Coordinate on one bounded task, keep changes
small, and keep acceptance evidence at the level it was actually observed.
