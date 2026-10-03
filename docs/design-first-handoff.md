# Design-first handoff example

This page shows how to turn one real, still-unapproved design request into a
pending core-game draft with the existing `game-studio brief` CLI, and how the
downstream check refuses that draft because no human has approved it yet. It is
a portable example: the brief commands run locally from repository files and
write their draft output only inside an artifact root you choose. Building the
CLI requires Go 1.26.2; a cold module cache may require dependency downloads,
as explained in [Contributing](../CONTRIBUTING.md#build-and-test).

The example request is an actual design-role return, preserved verbatim at
[`testdata/design-first-handoff/request.json`](../testdata/design-first-handoff/request.json).
It is a proposal for human review. Nothing here approves it, executes an engine,
or grants production authority.

## What this example is and is not

- It **is** a worked example of the accepted `brief draft` and `brief check`
  routes over the existing `internal/workflows/coregame` library.
- It **is not** an approved brief, a design decision, a Godot build, a G7
  (installed) check, or a full-v1 completion claim.
- The request, the generated draft, and any recorded decision are separate
  artifacts with different authority. Only a real human decision, observed
  independently by a coordinator, can unblock the downstream check.

## Quick path

Run from an isolated checkout with no ancestor workspace or `openspec/config.yaml`
marker above it, so no test or command can resolve a managed configuration
through a parent directory.

```sh
# 1. Build and explicitly select the local CLI, not a binary from PATH.
go build -o ./game-studio ./cmd/game-studio

# 2. Create the output parent and generate one pending draft.
mkdir -p artifacts/design
./game-studio brief draft \
  --input testdata/design-first-handoff/request.json \
  --root artifacts \
  --out design/draft.json

# 3. Create artifacts/decision-pending.json using the actual draft values.
# Follow "Pending decision probe" below; do not leave its placeholders unchanged.

# 4. Check that synthetic pending decision. This is expected to fail.
./game-studio brief check \
  --input artifacts/design/draft.json \
  --root artifacts \
  --decision artifacts/decision-pending.json
```

`--root artifacts` must already contain `artifacts/design/`. The CLI never
creates missing output directories. Because `brief draft` creates the output
with exclusive semantics, a second run with the same `--out` fails rather than
overwriting the first artifact.

## The example request

`testdata/design-first-handoff/request.json` is a JSON `coregame.DraftRequest`
using the exported field names the library documents in
[core-game-briefs.md](core-game-briefs.md):

| Field | Value in this example |
| --- | --- |
| `Mode` | `zero-to-one creation` |
| `Phase` | `gdd-slice` |
| `Request` | the original unapproved DH-01 goal text |
| `Classification` | empty (creation drafts carry no classification) |
| `Target` | `Godot handoff` |
| `Contents` | the eight ordered `gdd-slice` caller-owned sections |
| `References` | empty (a new creation draft needs no prior artifact) |

The eight `Contents` sections are exactly the caller-owned `gdd-slice` template
sections in order: `game concept`, `game pillars`, `core loop`, `player
fantasy`, `mechanics brief`, `narrative brief`, `tone and mood`, and `story
constraints`. Machine-owned sections such as `metadata`, `approval state`, and
`auto_approved: false` are rendered from typed fields and the contract; caller
text can never supply or overwrite them.

The request uses the role's original capitalized key spelling. JSON field
matching in Go is case-insensitive for exported names, so these keys decode to
the same `DraftRequest`, `SectionContent`, and `ArtifactReference` fields.

## Command reference

Both subcommands require all of their flags. There is no default root, no
working-directory scan, and no fallback root.

| Command | Required flags | Behavior |
| --- | --- | --- |
| `brief draft` | `--input`, `--root`, `--out` | Builds one JSON `coregame.Draft` inside `--root` at the root-relative `--out` path. |
| `brief check` | `--input`, `--root`, `--decision` | Re-reads reference bytes under `--root` and validates the draft against the supplied `coregame.HumanDecision`. Writes no artifact. |

### `brief draft`

- Always writes `Status: "draft"`, `ApprovalState: "pending_human_approval"` and
  `AutoApproved: false`, plus the canonical `Markdown` and a deterministic
  `Revision` digest. A draft is never approved by construction.
- `--out` must be a clean, root-relative path. Absolute paths, traversal,
  backslashes, `..`, and `.` are rejected before any output is touched.
- Existing output is never overwritten, including a dangling symlink. The
  output is created exclusively.
- Missing output parent directories are **not** created implicitly. Create the
  directory yourself, outside the CLI.
- The request is fully decoded and validated before the output is opened. A
  validation failure leaves inputs unchanged and creates no output.

### `brief check`

- `--input` is a JSON `coregame.Draft` (as written by `brief draft`), and
  `--decision` is a separate JSON `coregame.HumanDecision`.
- `--root` is opened with the standard library's confined `os.Root`, so
  reference reads cannot escape it through `..`, absolute paths, or symlinks.
- Check mode writes no artifact and performs no production action.

## Pending decision probe (expected negative check)

To prove that a pending draft stays blocked, supply a deliberately synthetic
pending decision that is clearly not a real decision. Substitute the actual
`target` and `revision` values read from the generated draft:

```json
{
  "Reference": "synthetic-pending-check-not-a-real-decision",
  "State": "pending",
  "Target": "<target from the generated draft>",
  "DraftRevision": "<revision from the generated draft>"
}
```

With that probe, `brief check` exits nonzero and prints exactly:

```text
brief check: downstream consistency not established: decision is still pending human approval (limitation: consistency validation only: verifies a draft against a coordinator-supplied recorded human decision; it does not authenticate a human, capture consent, or grant execution authority)
```

This nonzero result is the correct outcome for an unapproved draft. The probe is
a fixture, never an approval, and it must not be replaced with a synthetic
`approved` decision.

## Trust boundary

`brief check` is **consistency validation only**. It verifies that a draft
agrees with a recorded human decision that a trusted coordinator already
observed. It does **not** authenticate a human, capture consent, or grant
execution authority. The same limitation is returned with every result.

An absent, pending, declined, malformed, stale-reference, or target/revision
mismatched decision always fails. Approval is never inferred from draft text,
defaults, or convenience flags. A real coordinator must observe the human
decision independently before using such data for production.

## What this example does not establish

- No Godot or Blender execution, engine run, or 3D scene work.
- No G7 installation or compatible-reuse evidence.
- No full-v1 completion, and no human acceptance of the proposed design.
- No generated canonical draft, log, or decision is committed here. Generated
  output is local and disposable under the
  [generated artifact policy](generated-artifacts.md).
- The example request remains unapproved; an observed human decision is
  required before any downstream use.

## Related documentation

- [Core game briefs](core-game-briefs.md) — the library and CLI contract.
- [Generated artifact policy](generated-artifacts.md) — what may be versioned.
- [Project status](project-status.md) — the current capability checkpoint.
