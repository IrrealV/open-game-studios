# MCP project configuration (`mcp-config`)

`game-studio mcp-config --spec <spec.json>` turns one explicit local JSON spec
into a create-only Pi project MCP configuration at `WORKSPACE/.pi/mcp.json`. It
validates paths statically and never starts Pi, an MCP server, an interpreter,
or an engine. Write this file only for a project you trust: Pi loads
`.pi/mcp.json` after project trust is granted.

## Quick path

1. Provision the runtimes and roots yourself; this command creates no runtime
   directories, downloads nothing, and never probes an engine.
2. Write a local spec with explicit absolute paths (example shape only):
   ```json
   {
     "bootstrap_python": "/abs/python",
     "launcher": "/repo/tools/mcp/ogs_mcp_launch.py",
     "servers": [
       {
         "name": "ogs-blender",
         "mode": "blender",
         "interpreter": "/abs/venv/bin/python",
         "entrypoint": "/repo/tools/mcp/blender_headless.py",
         "engine": "/abs/blender",
         "project_root": "/abs/project",
         "runtime_root": "/abs/runtime",
         "output_root": "/abs/output",
         "description": "optional"
       }
     ]
   }
   ```
3. Run `game-studio mcp-config --spec <spec.json>` from inside the workspace,
   then inspect `.pi/mcp.json` and opt in per project through Pi's own trust
   flow. This command grants no trust and enables nothing.

## Spec contract

| Field | Rule |
|-------|------|
| `bootstrap_python` | Absolute path to an existing executable that runs the launcher. |
| `launcher` | Absolute path to the existing launcher file. Executable bit is not required. |
| `servers` | One or more entries; every requested server is emitted together. |
| `name` | `[A-Za-z0-9_-]+`; duplicates and `-`/`_` namespace collisions are refused. |
| `mode` | `blender` or `godot`. |
| `interpreter` | Absolute existing executable. |
| `entrypoint` | Absolute existing regular file; executable bit is not required. |
| `engine` | Absolute existing executable. |
| `project_root`, `runtime_root` | Absolute existing directories. |
| `output_root` | Required for `blender`; refused for `godot`. `null` is accepted for `godot` as omitted; `null` for `blender` is refused. |
| `description` | Optional; a safe default is emitted when omitted. |

`runtime_root` must already contain the child directories the launcher
preflights: `home`, `tmp`, `config`, `cache`, `data`, `state`, and `run`. A
missing, symlinked, or non-directory child is refused. A `godot` server additionally
requires a regular, non-symlinked `project.godot` marker directly inside
`project_root`. This command never provisions, creates, or repairs any of these
paths.

`output_root` is nullable on purpose: the launcher models a missing output root
as `None`, so an explicit JSON `null` is treated exactly like omitting the field
— allowed for `godot` (which emits no `--output-root`) and still refused for
`blender`, which requires a real output root. A non-null `output_root` on a
`godot` server is refused.

Unknown fields, a second JSON value, and trailing data are refused; the route is
Go's `encoding/json` decoding plus an unknown-field check, so field names are
matched case-insensitively and duplicate keys are not promised to be rejected.
Relative or missing paths, symlinked path components, non-regular files, and
non-directories are all refused. No environment expansion, `PATH` lookup, or
version inference happens.

Every emitted entry uses `command: /usr/bin/env` with
`args: ["-i", "PATH=/usr/bin:/bin", <bootstrap_python>, "-B", <launcher>, ...]`,
`enabled: false`, and no `env` block or credentials.

## Output rules

- The target is fixed: `WORKSPACE/.pi/mcp.json`. There is no `--out`.
- The workspace is the nearest ancestor with `.game-studio/workspace.manifest.json`,
  `.game-studio/workspace.config.json`, or `openspec/config.yaml`. A marker must be a
  regular file with no symlinked component; a symlinked or directory marker is
  refused. With none, the command refuses and writes nothing.
- Create-only: an existing `.pi/mcp.json`, even empty, is refused untouched; the
  file is never merged or overwritten.
- A new `.pi` directory is created `0700`; an existing real `.pi` keeps its
  permissions and state.
- The file is staged as a random `0600` temp inside `.pi`, then published with an
  exclusive hard link, so a concurrently created or racing target is never
  overwritten. Only this command's own staging temp is ever removed; the target is
  never removed or rolled back. If the target was published but removing the
  staging temp then failed, the command reports that cleanup failure, states the
  target is published and intact, and leaves the staging temp in place.
- Publication is not transactional with directory creation: if a new `.pi` was
  created and publication then fails, that empty or partial `.pi` may remain. The
  command never deletes a `.pi` or a target it did not prove it owns.

## Boundaries

- Static validation is not a sandbox and is not race-free; final execution
  authority still belongs to the launcher's own `LaunchPlan` checks. The emitter
  checks the same static preconditions (runtime children, Godot `project.godot`)
  but cannot eliminate time-of-check/time-of-use races.
- The emitted Blender entry runs trusted, unsandboxed `bpy`; `output_root`
  supplies HOME/XDG/tmp, not artifact confinement.
- The emitted Godot entry cannot confine upstream tools through `cwd`, because
  they accept a `projectPath` per call; use a dedicated session, not shared
  routing.
- Linux/WSL is the first supported platform. No other platform or release
  behavior is claimed.

## Checklist

- [ ] Every spec path is absolute, existing, and symlink-free.
- [ ] `runtime_root` already contains `home`, `tmp`, `config`, `cache`, `data`, `state`, `run`.
- [ ] A `godot` `project_root` contains a regular `project.godot`.
- [ ] `output_root` is present for Blender and absent or `null` for Godot.
- [ ] You accept that this file grants no trust and enables no server.
- [ ] You reviewed `.pi/mcp.json` before granting project trust in Pi.

## Next step

Read [docs/mcp-persistent-launchers.md](mcp-persistent-launchers.md) for the
launcher's validation and trust model, then complete Pi's project-trust step
yourself.
