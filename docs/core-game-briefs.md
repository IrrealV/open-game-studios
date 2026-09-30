# Core game briefs

`internal/workflows/coregame/brief.go` turns a structured request into a
concrete, still-unapproved core-game draft, and separately validates that a
draft is canonically coherent, complete enough, and consistent with an
out-of-band human decision before downstream handoff. It reuses the existing
`core-game-workflow/v1` vocabulary — `ModeContract` IDs, `PhaseID` values,
`BriefContract` fields, `MarkdownTemplate` sections and approval defaults —
instead of introducing a second schema.

## Quick start

```go
draft, err := coregame.BuildDraft(coregame.DraftRequest{
    Mode:           "repair/change handoff",
    Phase:          coregame.PhaseChangeBrief,
    Request:        "Adjust the approved core loop.",
    Classification: coregame.ClassificationDesignChange,
    Target:         "Godot handoff",
    Contents: []coregame.SectionContent{
        {Section: "requested change", Body: "..."},
        {Section: "intended outcome", Body: "..."},
        {Section: "affected systems/files", Body: "unknown at this stage."},
        {Section: "proposed options", Body: "..."},
        {Section: "risks", Body: "..."},
        {Section: "acceptance criteria", Body: "..."},
        {Section: "verification plan", Body: "..."},
    },
    References: []coregame.ArtifactReference{{
        Phase:    coregame.PhaseGDDSlice,
        Locator:  "artifacts/gdd-slice.md",
        Revision: "<sha256 of the current bytes>",
    }},
}, callerOwnedFS)
```

`BuildDraft` always produces `Status: "draft"`,
`ApprovalState: "pending_human_approval"` and `AutoApproved: false`. It never
invents design prose, approvals, engine actions or production side effects.

## Entry points

The four entry kinds map onto the three existing `ModeContract` IDs:

| Entry kind | Mode | Phase |
| --- | --- | --- |
| `creation` | `zero-to-one creation` | defaults to `gdd-slice` |
| `direct-phase` | `direct phase invocation` | any of the nine design phases |
| `change` | `repair/change handoff` | `change-brief` |
| `repair` | `repair/change handoff` | `repair-brief` |

Change and repair keep their own existing `PhaseID`s and brief contracts, so a
change brief renders `requested change` / `intended outcome` / `proposed
options`, while a repair brief renders `observed behavior` / `expected behavior`
/ `proposed fix options`. Direct phases that have no `MarkdownTemplate` render
the `PhaseID` plus the supplied content as literal blocks, without fabricating a
new phase template or a GDD-shaped questionnaire.

## Two completeness levels

Draft construction and downstream handoff enforce different completeness:

- **Draft minimum** (enforced by `BuildDraft`): a small set of caller-owned
  sections per templated phase; a non-templated direct phase needs at least one
  supplied block. A draft at this level is explicitly pending and is not
  handoff-ready.
- **Downstream completeness** (enforced by `ValidateDownstream`): every
  applicable caller-owned canonical section of the phase's template must be
  supplied (for `gdd-slice` that includes explicitly supplying minimal or
  no-narrative constraints), and a change or repair handoff must carry at least
  one byte-verified relevant artifact reference. A new creation draft needs no
  prior references.

This split lets an incomplete pending draft exist without being mistaken for an
approved handoff.

## Determinism and rejection

There is no natural-language inference. `BuildDraft` rejects an empty request,
an unknown or mismatched mode/phase, an invalid classification, an unknown
downstream target, a missing draft-minimum section, an empty or duplicate
section, a machine-owned section supplied as caller text, and a missing,
unreadable, malformed, duplicate, unknown-phase or stale artifact reference.
Errors name the offending field.

`MarkdownTemplate` sections that are machine-owned (`metadata`,
`classification`, `relevant artifacts`, `human decision`, `approval state`,
`auto_approved: false`, `downstream handoff target`, `downstream references`)
are rendered only from typed fields and the contract. Caller section text can
never overwrite them.

## Untrusted text rendering

Caller-supplied section bodies, the request text and reference locators are data,
never document structure. They are rendered as fenced literal blocks whose fence
is longer than any backtick run they contain, so arbitrary embedded fences cannot
break out and caller text cannot masquerade as Markdown headings, machine
approval metadata or raw executable HTML. The original text is preserved
verbatim in the revision digest. The machine-owned scope (request, selected
target and references) is rendered in `metadata` for every mode, and the gdd
`downstream references` section renders the contract's catalogue explicitly
labelled as a catalogue, not as the draft's selected target.

## Reference identity

Every `ArtifactReference` carries the existing artifact `PhaseID`, an explicit
caller-root-relative `Locator`, and the expected SHA256 `Revision`. `BuildDraft`
reads the locator through the caller-supplied `io/fs.FS` and compares the digest
against the actual bytes. A declared revision is never trusted on its own. Phase
is a label, not an identity: a direct phase revision may legitimately reference
prior bytes of the same phase at a declared locator.

The read seam is caller-owned. This package performs no filesystem, network or
discovery access of its own beyond the exact locators the caller declares: the
caller defines the root and the readable scope.

## Downstream consistency validation

`ValidateDownstream(draft, refs, decision)` first proves the draft is canonically
coherent: its `Version`, `Kind`, `Phase`, semantic fields and rendered Markdown
must match a canonical reconstruction of the same request, and its revision must
be that reconstruction's revision. A freshly recomputed digest over tampered
fields, or a matching synthetic decision, cannot rescue a malformed or
incoherent draft. It then rechecks the current reference bytes, enforces
downstream completeness, and requires an out-of-band `HumanDecision` that names
a nonempty recorded reference, carries an `approved` state, and is bound to the
exact draft revision and intended downstream target. An absent, pending,
declined, malformed or mismatched decision blocks, as does any post-build
mutation of a draft field, reference or the rendered Markdown. A true
`AutoApproved` or non-pending approval state always blocks. Document text that
claims approval is data and never bypasses the gate.

### Trust boundary

Downstream validation is **consistency validation only**. It verifies that a
draft agrees with a recorded human decision that a trusted coordinator already
observed. It does **not** authenticate a human, capture consent, or grant
execution authority. The same limitation is returned in every
`DownstreamValidation` result as `ValidationLimitation`, and a synthetic
decision used in tests is a fixture, not approval.

## CLI: `game-studio brief`

The CLI is a thin adapter over this package. It decodes caller-supplied JSON,
resolves one explicit artifact root, and delegates all construction and
validation to `BuildDraft` and `ValidateDownstream`. It generates no game
content, performs no engine/model/network execution, and never captures
consent.

```sh
game-studio brief draft --input <request.json> --root <artifact-root> --out <relative-draft.json>
game-studio brief check --input <draft.json> --root <artifact-root> --decision <record.json>
```

Both subcommands require all of their flags. There is no default root, no
working-directory scan, and no fallback root. Unknown subcommands, unknown or
mixed flags, and extra positional arguments are rejected.

### `brief draft`

`--input` is a JSON `coregame.DraftRequest` supplied by the caller (a future
design role will author it). The CLI reads it through the library's structured
fields, builds a draft with `BuildDraft` using the explicit `--root` as the
read seam, and writes exactly one JSON `coregame.Draft` artifact to `--out`.
The artifact always carries `Status: "draft"`,
`ApprovalState: "pending_human_approval"` and `AutoApproved: false`, plus the
canonical `Markdown` and the deterministic `Revision`. No separate Markdown
sidecar is written.

Input request fields (exported `coregame.DraftRequest` names):

| Field | Meaning |
| --- | --- |
| `Mode` | one of the three contract mode IDs |
| `Phase` | the target `PhaseID` (creation defaults to `gdd-slice`) |
| `Request` | the original request text, rendered literally |
| `Classification` | required only for change/repair |
| `Target` | a contract downstream target (required for change/repair) |
| `Contents` | ordered `{Section, Body}` blocks |
| `References` | ordered `{Phase, Locator, Revision}` artifact references |

Example request:

```json
{
  "Mode": "repair/change handoff",
  "Phase": "change-brief",
  "Request": "Adjust the approved core loop.",
  "Classification": "design_change",
  "Target": "Godot handoff",
  "Contents": [
    {"Section": "requested change", "Body": "..."},
    {"Section": "intended outcome", "Body": "..."},
    {"Section": "affected systems/files", "Body": "unknown at this stage."},
    {"Section": "proposed options", "Body": "..."},
    {"Section": "risks", "Body": "..."},
    {"Section": "acceptance criteria", "Body": "..."},
    {"Section": "verification plan", "Body": "..."}
  ],
  "References": [
    {"Phase": "gdd-slice", "Locator": "artifacts/gdd-slice.md", "Revision": "<sha256 of the current bytes>"}
  ]
}
```

The written artifact uses the exported `coregame.Draft` field names:
`Version`, `Mode`, `Kind`, `Phase`, `Classification`, `Request`, `Target`,
`Contents`, `References`, `Markdown`, `Status`, `ApprovalState`,
`AutoApproved`, `Revision`.

### `brief check`

`--input` is a JSON `coregame.Draft` (as written by `brief draft`), and
`--decision` is a separate JSON `coregame.HumanDecision` supplied by the
caller/coordinator. The CLI re-reads the reference bytes under `--root` and
runs `ValidateDownstream`. Check mode writes no artifact and performs no
production action; on success it prints the structured result and the
validation limitation.

| `HumanDecision` field | Meaning |
| --- | --- |
| `Reference` | nonempty recorded-decision reference |
| `State` | `approved` (only accepted state) |
| `Target` | must equal the draft target |
| `DraftRevision` | must equal the draft revision |

An absent, pending, declined, malformed, stale-reference or target/revision
mismatched input fails at the app boundary with a nonzero exit. It is never
reported as accepted, and approval is never inferred from draft text,
`--yes`-style defaults or convenience flags.

### Root bounds and output safety

- `--root` is opened with the standard library's confined `os.Root`, so
  reference reads cannot escape it through `..`, absolute paths or symlinks.
- `--out` must be a clean, root-relative path. Absolute paths, traversal,
  backslashes, `..` and `.` are rejected before any output is touched.
- Existing output is never overwritten, including a dangling symlink; the
  output is created with exclusive semantics.
- Missing output parent directories are not created implicitly. No install,
  chmod or directory creation is performed on user paths.
- The complete request and every reference are decoded and validated before the
  output is opened. A validation failure leaves existing input and reference
  bytes unchanged and produces no output file.
- A late write or close failure returns non-success and names the newly created
  incomplete artifact. Atomicity is not claimed and unrelated data is never
  cleaned.
- The complete encoded JSON draft, including its trailing newline, must fit
  within the same 1 MiB bound `brief check` reads. An oversized draft fails
  before any output exists, so `brief check` never rejects the CLI's own
  artifact; reduce the supplied content instead.
- Report delivery errors are returned rather than ignored. A draft report
  failure leaves the already written artifact in place and names it, and a check
  report failure returns an error without implying a failed validation or
  granting authority.
- Untrusted report fields (the output locator and the decision reference) are
  escaped with Go quoting, so a newline or control character cannot forge an
  extra `[brief]` report line.
- JSON reads are bounded (1 MiB), accept exactly one value, and reject unknown
  fields.

### Human-trust limitation

`brief check` verifies the consistency of a **claimed** external recorded
decision only. It cannot authenticate authors, capture consent, issue
permission or grant authority. A real coordinator must observe the human
decision independently before using such data for production. Real design-role
integration remains future work within V1-02; this slice does not install,
write native or repository metadata, or claim real human acceptance.
