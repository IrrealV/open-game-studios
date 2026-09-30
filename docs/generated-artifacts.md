# Generated artifact policy

Treat generated output as local and disposable unless this policy explicitly allows it to be versioned.

## Classification

### A — Authoritative versioned sources

These are reviewed sources of truth:

- Go sources and contracts
- Embedded templates
- OpenSpec specifications and configuration
- `SKILL.md` files
- Canonical documentation, including `GAME-STUDIO.md`

Change these sources intentionally and keep their contracts consistent.

### B — Deliberately versioned derived artifacts

`.atl/skill-registry.md` may be versioned only when it is current, reproducible, path-safe, and validated as an index. Today it is **machine-local**: it indexes absolute paths outside this repository, so it is excluded from the public snapshot until a portable regeneration exists. `.atl/` is therefore not a public setup dependency, and a public clone does not need it to build or contribute.

Future fixtures or golden files may be versioned only when regeneration is deterministic, the update command is documented, and tests detect stale output. Normalize timestamps and other unstable values before adopting a golden.

### C — Local unversioned artifacts

Keep these artifacts out of version control:

- `.atl/.skill-registry.cache.json`
- `.codegraph/`
- `.game-studio/generated/`
- `.game-studio/workspace.manifest.json`
- `.game-studio/workspace.config.json`
- `profiles/game-studio/generated/`
- Dogfood outputs, reports, and temporary coverage files

> `.opencode/profiles/` is a legacy unversioned location retained for historical
> inputs only; the current first-class workspace anchor is `.game-studio/`. Do not
> create new tracked `.opencode/` artifacts, and do not delete old generated files
> that are explicitly kept as history.

## Runtime roles and authority

| Artifact | Role |
| --- | --- |
| Workspace manifest | Workspace identity and topology. |
| Workspace configuration | Mutable, confirmed preferences and the local setup-preset authority. |
| Final generated artifact | Execution receipt and snapshot of resolved inputs. |

Generated outputs never silently become configuration sources. Setup preset precedence is explicit invocation, then confirmed workspace configuration, then the `recommended` default. The routing preset is derived from the resolved setup preset in the current flow. Generated artifacts must retain high-level source metadata such as explicit, persisted-workspace, default, or derived provenance; that metadata explains resolution but does not create new authority.

Flat generated profile artifacts use `studio-profile.<engine>.md` and
`studio-profile.<engine>.summary.md`. The pack layout keeps `profile.md` and
`summary.md`. Older `opencode-profile.<engine>.md` files that remain on disk are
historical and are not renamed or deleted.

## Public snapshot boundary

A public snapshot is an **explicit, reviewed allowlist**, not "everything except
the ignore file". `.gitignore` is only defense-in-depth for future untracked
writes; it is not the inclusion manifest. The public candidate still needs its
own reviewed list of included paths and a new history boundary before any push.

**Included by intent** — source, tests, embedded templates and resources,
canonical docs, this policy, the two reviewed project role definitions under
`.pi/agents/`, and the first-party `LICENSE`.

**Excluded from the public snapshot** (private or machine-local):

- Raw memory exports, including `engram-export*.json` anywhere.
- Frozen private handoffs: `OGS_PROJECT_FOLLOWUP.md`, `OGS_HANDOFF_PI.md`,
  `OGS_HANDOFF_PI.txt`, and their Windows `*:Zone.Identifier` sidecars.
- Private task/history trees: root `odd/` and `research/InvestigacionOGS/`.
- Local engine/runtime session state, for example
  `testdata/ogs-speed-run-001/.runtime/`.
- Other local `.pi/` runtime state (for example
  `.pi/gentle-ai/sdd-preflight.json`), except the two reviewed project roles.
- Caches, credentials, cookies, provider/session state, and machine-specific
  context not listed above.

The exclusions above are **not exhaustive**: they are illustrative guards, and
they grant no permission to include any unlisted path. Excluding something also
never authorizes deleting a historical file. Every path that enters the public
snapshot must appear on an explicit, reviewed allowlist; there is no
default-include for paths this policy does not name. Ignore rules applied to
already-tracked files do not untrack or remove them, and historical private
files remain in private history.

## Operating rules

- Do not run `git add .` around generated outputs; stage reviewed paths explicitly.
- A tracked generated artifact requires an explicit reason, supporting tests, and a documented deterministic regeneration command.
- Keep dogfood runs under temporary or local-only paths.
- Never retain stale tracked profiles.
- Treat `.gitignore` as defense-in-depth only. Adding or narrowing a rule must
  not untrack, remove, or rewrite an already-tracked file, and it does not
  replace the reviewed public inclusion manifest.
- Do not weaken human approval gates or privacy controls when adjusting which
  artifacts are versioned.
- Keep authoritative sources and templates aligned with generated output; fix the source or template, then regenerate.
- Normalize timestamps and other nondeterministic fields before adopting fixtures or goldens.
