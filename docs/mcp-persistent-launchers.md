# Persistent MCP launcher (source slice)

`tools/mcp/ogs_mcp_launch.py` is a stdlib-only launcher that validates explicit
absolute paths for one managed MCP runtime, changes to the project directory,
then `execve`s the runtime once to inherit the open stdio pipes. It is a
**source slice**: it provisions, installs, starts no engine, touches no Pi config.

## Quick path

1. Provision the roots yourself; the launcher never creates, truncates, or
   overwrites anything:
   ```text
   <runtime-root>/{home,tmp,config,cache,data,state,run}   # must already exist
   <project-root>/                                          # Blender or Godot project
   <output-root>/                                           # Blender mode only
   ```
2. Launch from Pi with a clean bootstrap environment and explicit CLI args (no
   shell interpolation, no personal config). Placeholders only:
   ```bash
   /usr/bin/env -i PATH=/usr/bin:/bin \
     <python-absolute> -B <repo>/tools/mcp/ogs_mcp_launch.py blender \
     --interpreter <python-absolute> \
     --entrypoint <repo>/tools/mcp/blender_headless.py \
     --engine <blender-absolute> \
     --project-root <project-root> \
     --runtime-root <runtime-root> \
     --output-root <output-root>
   ```
3. Stderr carries diagnostics; stdout is reserved for the MCP protocol. Exit `2`
   means validation, `chdir`, or `execve` failed, with no retry or fallback.

## Modes

| Flag | `blender` | `godot` |
|------|-----------|---------|
| positional `mode` | `blender` | `godot` |
| `--interpreter` | server Python (absolute regular executable) | Node runtime (absolute regular executable) |
| `--entrypoint` | Blender MCP Python entrypoint | prebuilt Node index (pinned path) |
| `--engine` | Blender binary | Godot binary |
| `--project-root` | required; launcher `chdir`s here | required; launcher `chdir`s here; must contain `project.godot` |
| `--runtime-root` | required; owns server HOME/XDG/tmp | required; owns server HOME/XDG/tmp |
| `--output-root` | required | rejected |
| argv | `<interpreter> -B <entrypoint>` | `<interpreter> <entrypoint>` |
| extra env | `BLENDER_BIN`/`OGS_BLENDER_PROJECT_ROOT`/`OGS_BLENDER_OUTPUT_ROOT`, `PYTHONNOUSERSITE=1`, `PYTHONDONTWRITEBYTECODE=1` | `GODOT_PATH` |
| fixed env | `HOME`/`XDG_*`/`TMP*` under runtime root, `PATH=<interpreter-dir>:/usr/bin:/bin`, `LANG`/`LC_ALL=C.UTF-8` | same base env |

The child environment is a literal allowlist built from scratch: credentials,
proxy, GUI, loader, `PYTHONPATH`, `NODE_OPTIONS`, and harness variables are never
inherited.

**Two roots, two processes.** `--runtime-root` owns HOME/XDG/tmp for the MCP
**server** process the launcher execs. In Blender mode `--output-root` is a
separate root the adapter gives its **engine child** process, which creates
`output_root/runtime` itself. Do not point both at one path.

**Interpreter must stay the venv executable.** A standard venv symlinks
`bin/python` to the base interpreter, which the symlink-free launcher rejects.
The historical provisioning run used `python -m venv --copies` successfully so
`bin/python` is regular, then verify its prefix and deps. Never substitute the
base interpreter: that drops venv packages. An explicit resolved Node executable
is fine.

## Validation (static, before any exec)

Rejected with one actionable stderr line and nonzero exit, before `chdir`/`execve`:

- relative, missing, non-regular, non-executable, or symlinked interpreter/engine;
- missing, non-regular, or symlinked entrypoint;
- missing/non-directory/symlinked project, runtime, or output root;
- any missing or symlinked runtime `home/tmp/config/cache/data/state/run` dir;
- a Godot plan that passes `--output-root` (Blender-only);
- Godot project root without `project.godot`;
- Blender plan without `--output-root`.

## Trust model and limits

- **Static only, not a sandbox.** Checks run at validation time; no race-free or isolation guarantee.
- **Blender raw `bpy` is unsandboxed.** `output_root` supplies HOME/XDG/tmp, not
  artifact confinement; trusted scripts can write anywhere the user can.
- **Godot upstream tools are not project-pinned.** They accept a `projectPath`
  per call, so `cwd` does not constrain them; use a dedicated session, not shared routing.
- **`/tmp` is accepted here on purpose.** Unit fixtures and ephemeral runtimes
  are legitimate. Production provisioning/dispatch must refuse scratch or stale
  smoke paths; that policy lives in deployment config, not here.
- **Bootstrap env is a deployment contract.** Pi must start it with `/usr/bin/env -i`; the launcher cannot undo injected loader settings.
- **No health probe.** The launcher advertises none and runs no engine check.

## Checklist

- [x] All paths are absolute; the launcher rejects symlinked components.
- [x] Runtime `home/tmp/config/cache/data/state/run` and project/output roots are provisioned.
- [x] Server interpreter is a non-symlinked venv executable with verified deps.
- [x] Pi command uses `env -i` with explicit CLI args, not interpolation.
- [x] Historical Python checks: base 100 passed + 2 optional skips; venv 102 passed, 0 skipped.

## Historical provisioning evidence (local, disabled)

A bounded provisioning run created the managed runtimes and a disposable
validation project. It started no engine, MCP server, or Pi session and granted
no project trust; both configured entries are `enabled: false`.

- Blender 4.5.3 engine extracted fresh from the official Linux x64 archive
  (sha256 `975c58fc…a72d845`) under `$TOOLS/blender/4.5.3/`.
  `--version` reports `Blender 4.5.3 LTS`, and its host libraries resolve.
- Blender server venv at `$TOOLS/mcp/blender/venv` created in
  place with `python -m venv --copies`; `bin/python` is a regular non-symlinked
  executable whose `sys.prefix` is the venv. The 30-package hash-pinned wheel
  lock (sha256 `bc4658ca…e9a1d`, `mcp==1.26.0`) installed with
  `--require-hashes --only-binary=:all:`; `pip check` reports no broken
  requirements. No server was started.
- Godot package `@coding-solo/godot-mcp@0.1.1` installed under
  `$TOOLS/mcp/godot/` with `npm ci --ignore-scripts --omit=dev`
  from the reviewed 46-package lock (sha256 `e2cb3ecb…4baa624`). The installed
  `build/index.js` sha256 `fd3f5df6…38806dc0` matches the published archive
  member; no install scripts ran and no package was rebuilt.
- Disposable validation project `$WORKSPACE` holds a
  4-file copy of the trusted Godot fixture, a pre-staged (unexecuted) Blender
  build script, an engine output directory, and `home/tmp/config/cache/data/state/run`
  runtime trees for both servers. `.pi/mcp.json` is create-only (`0600`) with
  two `enabled: false` entries.

Both entries loaded as static `LaunchPlan` objects (real paths, environment
allowlist, `cwd`) without any `execve`; the launcher's symlink checks pass on
both interpreters. This validates the local configuration shape only. The
`command`/`args` used the source launcher under `tools/mcp/`, now recorded in
public commit `4c6e753`; configuration emission followed in `db1e98e`. These are
source-chain facts, not installed-release or product acceptance. `$TOOLS` and
`$WORKSPACE` above are portable placeholders, not paths a clone provisions.

## Next step

The disabled provisioning snapshot above is historical. Subsequent human-reported
native Pi health and Blender execution have separate evidence in the
[MCP checkpoint](mcp-config.md#evidence-checkpoint); trust is still each user's
explicit decision, never inherited from that run. MCP-3 and broader production
acceptance remain pending; no clean installer or global binary upgrade is claimed.
