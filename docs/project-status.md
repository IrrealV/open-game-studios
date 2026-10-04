# Project status

This is the authoritative checkpoint for **where Open Game Studios (OGS) stands
today**. It separates source-level acceptance from loader/delegation, engine
execution, human acceptance, installation, and device evidence. It is not a
release certification, and it deliberately carries no freshness date or
certification claim. Public availability is separate from release readiness
(see [Publication status](#publication-status)).

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
| Design specialist role (`.pi/agents/ogs-design-specialist.md`) | Present | Invoked once for a creation handoff | n/a | One creation brief approved | Not run | n/a |
| Core-game brief library (`internal/workflows/coregame`) | Accepted | n/a | n/a | n/a | n/a | n/a |
| `game-studio brief draft` / `brief check` CLI | Accepted | n/a | n/a | Fixtures plus one observed creation approval (consistency only) | n/a | n/a |
| Wizard plan-only preview | Accepted | PTY-exercised | n/a | Human accepted | Not run | n/a |
| Installer backend (detect / plan / consent / execute) | Accepted | Not run | Not run | Not run | Pending (G7) | n/a |
| Godot 2D movement slice | Accepted | n/a | Historical observed | Historical human playtest | Not run | n/a |
| Godot 3D scene work | Not implemented | Not run | Not run | Not run | Not run | Not run |
| Native Pi MCP configuration + launchers | Focused checks passed | Human-reported: 2 Blender / 14 Godot tools | Health for both; disposable Blender cube/export | Execution reported, not artistic approval | Not run | Not run |
| Blender production asset + export | Foundation only | No studio-role handoff | Disposable cube verified; not V1-05 | Artistic approval not run | Not run | Not run |
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
| Game design | Read-only design role invoked once for a creation brief; one human approval observed and consistency-checked. |
| Art direction | Staged. Contracts exist as metadata; no operational role. |
| Blender production | Native MCP execution foundation exercised; production-role handoff and artistic approval pending. |
| Godot implementation | Read-only diagnosis role used once; no 3D execution slice. |
| Audio / music / SFX | Staged. Contract-level only; no playback route. |
| Independent game QA | Staged. Distinct from code/native review and human playtest. |

## Native MCP foundation checkpoint

The [MCP guide](mcp-config.md#evidence-checkpoint) records source provenance,
human-reported Pi execution, independent current-content checks, and retained
failures separately. This foundation does not depend on Gentle Shell for MCP.
MCP-3 remains **pending**, not complete: source configuration and one prepared
workspace do not certify installation, studio routing, or the full journey.

## Historical evidence (labelled as historical)

- A Go suite previously passed with **14 packages green and one package with no
  test files**, and formatting was clean. This was not re-run for the current
  documentation-only update; it is historical suite evidence, not a fresh check.
- A 2D Godot fixture was executed headlessly and received a human playtest.
  This supports the 2D movement behavior only; it is not 3D, audio, Blender,
  device, or full-journey evidence.
- Three bounded native reviews were completed and their authority consumed for
  the design-handoff delivery (DH-01, DH-02, DH-03). Earlier candidates were
  historically unavailable or blocked. Independent technical checks do not
  substitute for native review.
- Historical evidence describes earlier repository states. It must not be
  promoted to current product readiness.

## Current next task

**V1-03 — implement and check a small Godot 3D task.** The accepted design cut
is the input: a bounded, disposable 3D scene or script with explicit criteria
and independent checking. It is not another design interview and not a
full-platform certification.

**V1-02 — make design and its existing contracts executable — is accepted at
bounded source-local scope.** The read-only design role was invoked once for a
creation request and returned an eight-section `gdd-slice` creation proposal
(a `coregame.DraftRequest`), not an approval or an executable plan. The `brief`
CLI built once, produced the canonical pending draft from that proposal once,
and then accepted the downstream consistency of an observed human approval of
that exact One Small Reach creation brief, once, with no timeout. The pending
draft kept `Status: draft`, `ApprovalState: pending_human_approval`, and
`AutoApproved: false`; those fields are never the approval. The CLI's acceptance
is **consistency validation only**: it does not authenticate a human, capture
consent, or grant execution authority. The library and CLI support creation,
direct-phase, change, and repair routes, but only this one live creation journey
was observed; the other routes rest on historically accepted source and
fixtures, not live runs. The committed example request and the
[design-first handoff guide](design-first-handoff.md) are a frozen, pending
drafting example with no approval record attached. The separate observed human
decision was applied to that exact content in one guided run; the example alone
is not approval or execution authority and cannot be inherited by another user
or run.

## Publication status

This repository is public and open for contributions as a work in progress.
Main branch protection is active. Publication is not clean-install or
release certification, and no new publication checks are claimed here.
This page reflects the
current source checkpoint, not a finished release or installed product.

## Known limitations

- This passive documentation update runs no build or engine. Earlier focused
  builds and engine checks are recorded separately in the MCP guide; no real
  installation or full-journey acceptance is established.
- The six responsibilities are staged; the Godot and design roles each have one
  recorded real bounded invocation.
- Blender production, audio, independent game QA, Godot 3D, and the Quest 3S
  route remain open, as does the full end-to-end continuation journey.
- Model identity, complete tool confinement, and multi-platform readiness are
  not established.
