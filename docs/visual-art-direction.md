# Visual art direction

`game-studio visual` turns human-authored visual direction into the
`visual-workflow/v1` metadata that future asset adapters can read. It creates
and validates an **Art Bible**, per-asset **Asset Specs**, and a **read-only
Asset Audit**. It is metadata only: it never executes ComfyUI or Blender,
generates an image or model, mutates a Godot/DCC file, covers audio, or approves
an Art Bible on its own.

The contracts come from
[`openspec/specs/visual-workflow/spec.md`](../openspec/specs/visual-workflow/spec.md)
and the shared vocabulary in
[`internal/assets/types.go`](../internal/assets/types.go). The implementation
lives in [`internal/workflows/visual`](../internal/workflows/visual) and the CLI
adapter in [`internal/cli/commands/visual.go`](../internal/cli/commands/visual.go).

## Quick path

1. Write an Art Bible, or seed one and edit it:

   ```sh
   game-studio visual art-bible design/art/art-bible.md
   ```

   A missing file writes a pending template; an existing file is validated.
2. Write one Asset Spec per asset:

   ```sh
   game-studio visual asset-spec design/assets/specs player-capsule
   ```

3. Audit the whole game root:

   ```sh
   game-studio visual audit .
   ```

4. Have a human review the Art Bible, then record approval explicitly:

   ```sh
   game-studio visual approve-art-bible design/art/art-bible.md \
     --confirm --reviewer "<human name>"
   ```

Expected result: `metadata_ready: true`. `adapter_ready` stays `false` until the
Art Bible is approved.

## What the documents are

| Document | Answers | Required content |
|---|---|---|
| **Art Bible** | What is this game's visual direction? | Visual pillars, style, palette, references, constraints, negative guidance, approval state |
| **Asset Spec** | What is this one asset, and how do we judge it? | Intent, gameplay use, narrative use, Art Bible links, output targets, prompt-ready metadata, import hints, acceptance criteria |
| **Asset Audit** | Do the documents agree and are they ready? | Read-only alignment checks and a readiness status |

Documents are markdown with YAML frontmatter. The frontmatter carries identity
and approval metadata; the body carries the structured sections the evaluator
reads.

```markdown
---
version: 1
approval_state: pending
---

## Visual Pillars
- Clean minimalism

## Palette
- Player: cyan (#00B3FF)
```

```markdown
---
version: 1
asset: player-capsule
art_bible: ../art-bible.md
approval_state: pending
---

## Output Targets
- Godot `CapsuleMesh` primitive (radius 0.3, height 1.6)

## Acceptance Criteria
- Player color is cyan #00B3FF.
```

## Commands

| Command | Reads | Writes |
|---|---|---|
| `visual art-bible <path>` | `<path>` when it exists | A pending template when `<path>` does not exist |
| `visual asset-spec <dir> <name>` | `<dir>/<name>.md` when it exists | A pending template when the file does not exist |
| `visual audit <game-root>` | `<game-root>/art-bible.md` and `<game-root>/assets/*.md` | Nothing |
| `visual approve-art-bible <path> --confirm --reviewer <name>` | `<path>` | Rewrites `approval_state` to `approved` |

Templates are created with exclusive semantics and never overwrite an existing
file. Parent directories are not created implicitly.

## Readiness rules

`visual audit` runs these checks and prints each one:

| Check | Blocks metadata readiness | Notes |
|---|---|---|
| `art_bible_fields` | Yes | All required Art Bible fields present |
| `asset_specs_present` | Yes | At least one Asset Spec found |
| `art_bible_links` | Yes | Every spec links the audited Art Bible, and resolves to it |
| `output_targets` | Yes | Every spec declares output targets |
| `prompt_import_metadata` | Yes | Every spec carries prompt metadata and import hints |
| `acceptance_criteria` | Yes | Every spec has testable acceptance criteria |
| `art_bible_approval` | No | Reported; approval gates adapter readiness, not metadata planning |
| `execution_boundary` | Yes | Confirms the audit stayed read-only |

Status is one of:

- `ready_for_metadata_planning` — documents are complete, Art Bible approval
  pending.
- `ready_for_adapter` — documents complete and Art Bible approved.
- `concerns` — at least one blocking check failed; the command exits nonzero.

## Approval workflow

Approval is a **human decision**, never a side effect of validation.

- `visual art-bible` and `visual asset-spec` only validate; they never approve.
- `visual approve-art-bible` requires both `--confirm` and a nonempty
  `--reviewer`. Without them it refuses and changes nothing.
- A recorded approval is a **claimed** human decision. The CLI does not
  authenticate the reviewer and grants no execution authority. A real
  coordinator must observe the human decision independently.
- Do not treat a draft `approval_state`, a template placeholder, or a synthetic
  fixture as approval.

## Integration with future Blender work (V1-05)

An Asset Spec's **output targets**, **prompt metadata**, and **import hints** are
the handoff surface for future adapter planning. They describe what a future
Blender/ComfyUI lane should produce and how it should be imported, without doing
any of it:

- Adapter readiness stays `false` until the Art Bible is approved.
- `internal/assets/catalog.go` marks `blender-reference-modeling` and
  `comfyui-workflows` as metadata-only adapters blocked on
  `visual-workflow/v1` readiness.
- V1-05 owns the real Blender execution, editable asset, and export; V1-06 owns
  Godot import and validation. Neither happens here.

## Example: One Small Reach fixture

The disposable 3D fixture under
[`testdata/one-small-reach-3d/`](../testdata/one-small-reach-3d) is the worked
example:

```sh
game-studio visual audit testdata/one-small-reach-3d
```

It reports `metadata_ready: true`, `adapter_ready: false`, and
`status: ready_for_metadata_planning`, because the Art Bible is still
`pending`. The captured report is
[`testdata/one-small-reach-3d/asset-audit.md`](../testdata/one-small-reach-3d/asset-audit.md).

The fixture currently renders engine primitives with slightly different
dimensions and close colors; the Asset Specs state the direction target for the
future Blender upgrade rather than claiming the engine already matches.

## Checklist

- [ ] Art Bible has pillars, style, palette, references, constraints, and negative guidance.
- [ ] Every Asset Spec links the Art Bible and declares output targets.
- [ ] Every acceptance criterion is measurable.
- [ ] `game-studio visual audit <game-root>` exits zero with `metadata_ready: true`.
- [ ] A human, not the CLI, approved the Art Bible before any adapter readiness is claimed.

## Next step

Review the Art Bible as a human. If it is right, record approval with
`visual approve-art-bible`; otherwise edit it and re-run `visual audit`. Adapter
planning (V1-05) stays blocked until approval is recorded.
