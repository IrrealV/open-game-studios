# Contributing

Thanks for your interest in Open Game Studios (OGS). Contributions are welcome
through pull requests.

## Before you start

1. Read [README.md](README.md) for the project shape.
2. Read [docs/project-status.md](docs/project-status.md) so you know what is
   actually working and at what evidence level.
3. Pick one bounded task from [docs/roadmap.md](docs/roadmap.md).
4. If the task or its scope is unclear, open an issue or a draft PR to discuss
   it before writing much code.

You do **not** need private issue references, a private task tracker, or any
memory system to contribute. Public continuity works from the files in this
repository.

## Contribution flow

1. Create a branch (see naming below). Do not work directly on the default
   branch.
2. Make one small, coherent change. Keep implementation, tests, and any
   documentation it changes together.
3. Run the relevant checks locally and capture the exact commands and results.
4. Open a pull request with a clear scope, what you changed, and what you
   verified — including anything you could **not** verify.
5. A maintainer performs human review. Changes land after review; nothing is
   merged on the strength of an automated or self-approved check alone.

Pull-request-based contribution is the default here for all changes, including
small ones. Large or high-risk work additionally benefits from an early design
discussion in the PR.

## Scope and size

Keep each pull request to one reviewable unit. Around 400 authored lines is a
review-planning guideline, not a hard cap: a coherent change may exceed it, but
do not split behavior artificially or minify code, tests, or documentation to
hit a number. Do not mix unrelated changes.

## Build and test

Go 1.26.2 is required, and Linux/WSL is the first supported platform.

```sh
go build ./cmd/game-studio
go test ./... -count=1
```

Honest cautions:

- The repository does not vendor Go modules, so on a cold module cache these
  commands download this module's dependencies from the configured module
  proxy.
- The tests are **not fully isolated from their surroundings**. Workspace
  resolution walks ancestor directories, so running the suite inside a checkout
  that has an ancestor workspace marker or an ancestor `openspec/config.yaml`
  can cause some wizard tests to write managed configuration through that
  ancestor instead of their temporary directory. Run the suite in an isolated
  copy of the checkout with no ancestor workspace, and review anything a run
  wrote.
- The `internal/cli/commands` tests install a no-op `engram` shim so tests never
  touch a personal memory database. That shim exists for test safety; it is not
  a setup or installation step, and it is not a substitute for engine or
  install testing.
- Do not add shell code whose only purpose is to bypass tests, skip isolation, or
  hide a failure.

Optional Node checks exercise the Godot fixture. Run the structural-only command
with `GODOT_BIN` explicitly empty — the source reads `GODOT_BIN` from the
environment only and never auto-detects an engine, so an inherited value could
otherwise launch Godot. The second command is an opt-in native Godot headless
example that requires an explicit engine binary:

```sh
GODOT_BIN='' node --test tests/pi-godot-change.test.mjs
GODOT_BIN=/absolute/path/to/godot4 node --test tests/pi-godot-change.test.mjs
```

These checks cover the selected movement behavior only, not visual quality, fun,
or full-game readiness.

## Reporting evidence

- Report the exact commands you ran and their observed results.
- List every failure, skip, and pending check honestly. Do not claim a check you
  did not run.
- Separate source-level, fixture-level, engine-level, and human evidence. A
  green fixture test does not prove a runtime composition works.
- If you need an external engine or provider, do not add installation
  instructions unless they are already documented and verified here and are
  clearly optional. Prefer pointing to the existing reviewed reference instead.

## Generated artifacts

Before generating or committing repository output, follow the
[generated artifact policy](docs/generated-artifacts.md). Stage reviewed paths
explicitly; never sweep generated output into a commit.

## Commit and branch conventions

These are the repository's existing conventions:

- Use [Conventional Commits](https://www.conventionalcommits.org/):
  `type(scope): description` or `type: description`.
- Allowed types: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
  `refactor`, `revert`, `style`, `test`.
- Lowercase feature branches matching
  `^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)\/[a-z0-9._-]+$`.
- No `Co-Authored-By` footer and no AI attribution in commits.

## Review and governance

- A maintainer reviews and merges contributions. Human review is required; do
  not merge your own change.
- No CLA or DCO is required, and none is implied.
- No CI workflow is configured in this repository today. Do not assume an
  automated pipeline will validate your pull request — report your own local
  evidence in the PR.
- Earlier internal verification used private, audited scratch harnesses. Those
  are historical internal evidence, are not part of this repository, and are not
  required (or available) for public contribution.
