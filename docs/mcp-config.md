# MCP project configuration (`mcp-config`)

`game-studio mcp-config --spec <spec.json>` turns one explicit local JSON spec
into a create-only Pi project MCP configuration at `WORKSPACE/.pi/mcp.json`. It
validates paths statically and never starts Pi, an MCP server, an interpreter,
or an engine. Write this file only for a project you trust: Pi loads
`.pi/mcp.json` after project trust is granted.

## Quick path

1. Provision the runtimes and roots yourself; this command creates no runtime
   directories, downloads nothing, and never probes an engine.
2. Write one local spec with explicit absolute paths. This single-server
   example shows the shape, not a separate command per server:
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
   For a joint configuration, add a `godot` entry to the same `servers` array
   with an explicit Node interpreter, prebuilt `build/index.js`, engine,
   project and runtime roots; omit `output_root` or use `null`. One invocation
   emits both entries together. A second invocation cannot append to the file.
3. Run `game-studio mcp-config --spec <spec.json>` from inside the workspace,
   then inspect `.pi/mcp.json` and opt in per project through Pi's own trust
   flow. This command grants no trust and enables nothing.

## Prerequisites and recovery

Use an explicitly built source CLI with Go 1.26.2; the existing local binary
was not upgraded or installed on normal PATH. Provision Python with the Blender
server dependencies, Node with the pinned Godot package, both engines, and the
root directories yourself; see [launcher evidence](mcp-persistent-launchers.md).
A missing-path diagnostic means correct the spec or provision that exact path
under separate authorization, not enable auto-installation or bypass validation.

Ordinary `init` also needs a valid legacy setup work document and an available
Godot command. The root `GAME-STUDIO.md` is not a compliant legacy init input.
The observed setup used a valid setup-only document and process-local HOME/XDG/
PATH with a Godot alias for a known `--version` command; it changed no global
PATH. This was real init, not a fake marker or an environment-check bypass.
Working-document validation is not approval of a core-game draft. Explicit
engine paths work for MCP once provisioned; normal-PATH discovery is not proven.
Inspect the generated configuration and grant trust/enable tools yourself in
Pi's UI. The emitter never grants trust or authorizes execution.

## Evidence checkpoint

These are bounded prior observations, not checks run by this passive update.

| Evidence class | Outcome and limit |
|---|---|
| Public source chain | `e971fa3`: adapted upstream Blender smoke; `ae52d0c`: Coding Solo SDK scene/node smoke; `c4d1ec1`: two-tool headless adapter; `4c6e753`: clean persistent launcher; `db1e98e`: create-only native Pi CLI emitter. |
| Historical source checks | Python: 100 passed + 2 optional skips on base interpreter; 102 passed / 0 skipped in venv; adapter constrained checks: 60. Focused Go MCP and CLI tests, build and formatting independently passed offline with Go 1.26.2. Default host Go 1.24 mismatch was reported; this is not every-host reproducibility. |
| Static prepared configuration | Joint disabled emission, actual public `LaunchPlan` validation, refusal preserving existing bytes, and invariants passed. This is a developer-prepared layout, not clean installation or Windows/macOS certification. |
| Human-reported native Pi | New CLI-generated workspace listed 2 Blender / 14 Godot tools. Health: Blender 4.5.3 LTS exit 0 without error; Godot 4.7.2 official. After confirming cwd and absent outputs, trusted Blender execution returned exit 0 without error, timeout or truncation and produced `.blend` + 1,992-byte GLB. No direct launch-to-source or cryptographic attestation is claimed. |
| Independent current content | Fresh reopen with auto-execution disabled: one origin cube, dimensions 2, 8 vertices / 6 polygons, `OGS_AdapterMaterial`, blue `[0.2,0.4,0.8,1]`, roughness 0.35. Full GLB decode passed: 24 positions/normals/UV VEC2, 36 ushort indices, 8 corners, bounds ±1, 12 triangles of area 2, PBR; JSON 1,124 + BIN 840, total 1,992 bytes. |
| Fresh prospective invariants | Separate run found zero differences across 210 tracked sources and all six source/asset/protected-project/binary/Git invariants; blend check passed. Its decoder then failed `KeyError: VEC2` (checker bug, not an asset defect). |
| Separate static completion | Decoder passed 7 positive/negative fixtures and the full parse without engines; file hashes and metadata differences stayed unchanged. That helper did not execute Git semantics in-period; parent checked current HEAD/status/unstaged/staged diffs separately. It is not an all-green combined run. |

Current content identifiers (not reproducibility promises for Blender bytes):
- `.blend`: 450,055 bytes; SHA256 `9af06a85970506aea96b025bf93c28e5fe00b2b31b0b775b1016e5a8e3aac062`.
- GLB: 1,992 bytes; SHA256 `c295ef5c8fc317d7b77be8be54cd60c351266419c693138b35186a713a4670bd`.

**Retained failures and pending acceptance.** The first generated-asset verifier
passed geometry/PBR/full GLB and asset immutability, then failed the public
metadata inventory (including Git and ignored local task state). No after
snapshot persisted: cause remains unknown and protected-root checks were skipped.
Later timestamps cannot explain that failure retroactively; fresh invariants are
prospective, not reconstruction. An earlier manual Pi check separately failed a
managed-cache/extra immutable-runtime assertion. Neither history is waived.
Tool-output truncation also created a temporary log outside the checker's output
child; its observed mode was 0644 and the human authorized only changing it to
0600, with no content read or deletion. This is an operational artifact scope
exception, not a new code crash.

MCP-3 remains **pending**. Studio/Gentle Shell role handoff, shared routing,
artistic approval, Godot scene mutation through Pi, Godot import, audio, VR,
human playtest, G7 and the full journey remain unobserved here. Historical SDK
scene creation is not native Pi scene-mutation evidence. The disposable cube
satisfies no V1 production gate and adds no Gentle Shell MCP dependency.

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
