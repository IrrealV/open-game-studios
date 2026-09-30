# Project status

This is the authoritative checkpoint for **where Open Game Studios (OGS) stands
today**. It separates source-level acceptance from loader/delegation, engine
execution, human acceptance, installation, and device evidence. It is not a
release certification, and it deliberately carries no freshness date or
certification claim. The publication checks for this documentation slice are
still pending (see [Publication status](#publication-status)).

## How to read the matrix

| Level | Means |
|---|---|
| Source & fixtures | Code/specs exist and source-level or fixture tests were accepted. |
| Loader / delegation | A host actually discovered and loaded a resource, or a real bounded handoff ran. |
| Engine execution | A real engine or host tool executed a behavior and produced evidence. |
| Human acceptance | A human observed and accepted the result. |
| Installed (G7) | A real installation or reuse ran on an authorized target. |
| VR / device | A headset build/export and device interaction were observed. |

"Accepted" means accepted at the stated level only. Source acceptance is not
installed-pack, engine, or whole-product acceptance.

## Capability evidence matrix

| Capability | Source & fixtures | Loader / delegation | Engine execution | Human acceptance | Installed (G7) | VR / device |
|---|---|---|---|---|---|---|
| `ogs-core` entry skill + handoff contract | Accepted | Observed once, source-local | n/a | n/a | Not run | n/a |
| `ogs-godot-change` bounded change skill | Accepted | Guidance consumed in the same bounded handoff | Historical 2D fixture only | Historical 2D playtest | Not run | Not run |
| Godot specialist role (`.pi/agents/ogs-godot-specialist.md`) | Present | Host-discovered and used once | Not run | Not run | Not run | Not run |
| Design specialist role (`.pi/agents/ogs-design-specialist.md`) | Present | Host-listed; not invoked | Not run | Not yet | Not run | Not run |
| Core-game brief library (`internal/workflows/coregame`) | Accepted | n/a | n/a | n/a | n/a | n/a |
| `game-studio brief draft` / `brief check` CLI | Accepted | n/a | n/a | Fixtures only | n/a | n/a |
| Wizard plan-only preview | Accepted | PTY-exercised | n/a | Human accepted | Not run | n/a |
| Installer backend (detect / plan / consent / execute) | Accepted | Not run | Not run | Not run | Pending (G7) | n/a |
| Godot 2D movement slice | Accepted | n/a | Historical observed | Historical human playtest | Not run | n/a |
| Godot 3D scene work | Not implemented | Not run | Not run | Not run | Not run | Not run |
| Blender asset + export | Not implemented | Not run | Not run | Not run | Not run | Not run |
| Audio integration | Not implemented | Not run | Not run | Not run | Not run | Not run |
| Independent game QA route | Not implemented | Not run | Not run | Not run | Not run | Not run |
| Quest 3S Android/OpenXR/hand interaction | Not implemented | Not run | Not run | Not run | Not run | Pending device gate |

## Installer and wizard

- The Go installer backend and the Pi-only wizard flow are accepted within
  **source and local-fixture scope**. Detection, planning, fingerprint-bound
  consent, cancellation/partial-failure reporting, and Engram write-through were
  checked with fakes and fixtures.
- The styled plan-only wizard preview was exercised through controlled terminals
  and accepted by a human for its visual and usability cut. The preview never
  installs, approves, or writes anything.
- **G7 — real installation or compatible reuse on an authorized Linux/WSL
  target — is not done.** Installed-pack behavior, real vendor downloads, and
  loaded-Pi acceptance remain unobserved. The published prerequisite versions
  are declared candidates, not a certified live composition.

## The six studio responsibilities

OGS defines six on-demand responsibilities. They are **responsibilities, not
six operational agents**, and the current source supports at most two read-only
project roles.

| Responsibility | Current state |
|---|---|
| Game design | Read-only design role file exists; not yet invoked or independently accepted. |
| Art direction | Staged. Contracts exist as metadata; no operational role. |
| Blender production | Staged. No execution route connected. |
| Godot implementation | Read-only diagnosis role used once; no 3D execution slice. |
| Audio / music / SFX | Staged. Contract-level only; no playback route. |
| Independent game QA | Staged. Distinct from code/native review and human playtest. |

## Historical evidence (labelled as historical)

- A Go suite previously passed with **14 packages green and one package with no
  test files**, and formatting was clean. This was not re-run for the current
  documentation-only slice, and it is not evidence about the nine documents
  changed here.
- A 2D Godot fixture was executed headlessly and received a human playtest.
  This supports the 2D movement behavior only; it is not 3D, audio, Blender,
  device, or full-journey evidence.
- Native review outcomes were historically **unavailable or blocked** for the
  relevant candidates. No native approval exists for any work described here,
  and independent technical checks do not substitute for it.
- Historical evidence describes earlier repository states. It must not be
  promoted to current product readiness.

## Current next task

**V1-02 — make design and its existing contracts executable.** The library and
CLI are accepted at source/fixture scope. What remains is an actual bounded
design-role handoff and an **observed** human approval gate before any
production step. This is not a stale design-writer task: the writer step for the
role file is done, and the remaining work is invocation and observed acceptance.

## Publication status

This repository is being prepared for a reviewed public snapshot. The rename of
the private repository, the private memory backup, the public-candidate assembly
check, and the publication itself are **not executed** by the current
documentation slice. Treat this page as the current checkpoint of the source
tree, not as a published release.

## Known limitations

- No fresh build, engine run, installation, or full-journey acceptance was
  performed for the current documentation slice.
- The six responsibilities are staged; only the Godot role has a recorded real
  bounded invocation.
- Blender, audio, independent game QA, Godot 3D, and the Quest 3S route remain
  open, as does the full end-to-end continuation journey.
- Model identity, complete tool confinement, and multi-platform readiness are
  not established.
