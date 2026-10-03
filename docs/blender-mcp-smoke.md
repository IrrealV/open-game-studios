# Blender MCP smoke — isolated headless execution evidence

Bounded runtime evidence for the MCP execution-validation task. This records an
**adapted-package headless smoke**: the published `blender-mcp-server` wheel was
driven over real MCP stdio, but its bridge (live Blender add-on) path was
deliberately blocked. It is **not** upstream startup verification, **not** live
editor control, **not** Pi-loaded integration, and **not** full V1 acceptance.

This document was corrected after an independent read-only review. Sections are
tagged with their evidence class: **[historical]** = initial worker's own run
metadata, not independently reproduced; **[correction]** = observed during the
bounded correction pass; **[independent]** = a separate read/verify process.

- Initial run: 2026-10-03 (host clock). Correction pass: 2026-10-04.
- Scratch root (ephemeral, kept for verification): `<smoke-root>`.
- Branch `feat/mcp-execution-validation`, base
  `f50a369a04c6c8c34122889ccfda6b57354d079e`. No commit, push, or PR.

## 0. Repo and tracker state (corrected) **[correction]**

The initial worker's "repo left unchanged" claim was false. Actual state:

- `docs/blender-mcp-smoke.md` was **added** by the worker (untracked); editing
  it is the only repository write in this task.
- `git status --porcelain` before and after the correction run:
  `?? docs/blender-mcp-smoke.md` and `?? game-studio` (the latter pre-existing,
  preserved untouched). No tracked source or configuration was modified.
- The `/odd/` work tracker is git-ignored, so tracker edits never appear in
  `git status`; the absence of `odd/` there is not evidence that it is unchanged.
- The private checkout was never touched by this worker.

## 1. Host preflight and engine compatibility **[historical] + [correction]**

`uname -m` → `x86_64`; 64-bit; no system libraries installed. The engine's
`ldd` output had **0** unresolved entries (56 lines), and `readelf -d` shows
`RUNPATH $ORIGIN/lib` for the engine binary.

**Correction to the earlier claim:** `RUNPATH $ORIGIN/lib` plus a clean `ldd`
proves the bundle is **compatible with this host**, not that it is
self-contained. The binary also `NEEDED`s host-provided libraries such as
`libX11.so.6`, `libXrender.so.1`, `libXfixes.so.3`, `libXi.so.6`,
`libxkbcommon.so.0`, and `libvulkan.so.1`, which `$ORIGIN/lib` does not cover and
which resolved from the system. A reproducible host with these shared objects is
a prerequisite.

## 2. Engine provenance **[historical] + [independent]**

| Item | Observed value |
| --- | --- |
| Archive | `https://download.blender.org/release/Blender4.5/blender-4.5.3-linux-x64.tar.xz` |
| Size | `377397328` bytes (matches the reported size) |
| SHA256 | `975c58fcb244273838534bba771e64ad87739216b0f9b39a888531a49a72d845` |
| Published checksum | re-fetched `blender-4.5.3.sha256` (HTTP 200, ordinary User-Agent); linux-x64 line matches; `sha256sum -c` → `OK` |
| Version | `Blender 4.5.3 LTS`, build hash `67807e1800cc`, build date `2025-09-09` |

Archive metadata returned HTTP 403 with the default client identity and was
re-fetched with an ordinary User-Agent. No access control was bypassed.

**Archive containment (fresh, read-only).** The correction pass streamed the
`.tar.xz` with ``tarfile`` and **no new extraction** and independently
reproduced the member census: 5,579 files / 860 dirs / 71 symlinks; 0 absolute
names; 0 names containing `..`; 0 hardlinks; 0 device/other members; 0
symlink targets that are absolute or escape the archive root. All sampled
symlinks are intra-`lib` relative links. Report:
`out/verify_archive_report.json`.

**Qualified limits.** The original 5,579/860/71 extraction figures were
worker-reported run metadata, not an independent check. This new check is also
**static**: it inspects the compressed member list and link targets; it does not
materialize a filesystem, so it does not prove write-time path confinement or
that no extraction tool would mis-handle a member. No extraction was performed
in this correction.

## 3. Dependency provisioning (wheel-only, hash-locked) **[historical]**

Interpreter: `/home/linuxbrew/.linuxbrew/bin/python3` → CPython `3.14.7`,
pip `26.2.1`, in a fresh venv under the scratch root. `uv` was absent.

Resolution was reviewed from a `pip --report` before installation:

```
pip --isolated --disable-pip-version-check install --dry-run --ignore-installed \
    --only-binary=:all: --no-cache-dir --index-url https://pypi.org/simple \
    --report $S/out/resolution.json "blender-mcp-server==0.2.0" "mcp==1.26.0"
```

Python 3.14 compatibility is **observed working for resolution and the exercised
route only** (`cffi`, `pydantic-core`, `rpds-py` resolved to `cp314` wheels).

| Item | Value |
| --- | --- |
| Lock | `out/requirements.lock` (30 packages), SHA256 `bc4658ca2962a7dee46f0182eaad7d695e072cdc60f973f22613c8fa4e6e9a1d` |
| `blender-mcp-server` | `0.2.0`, wheel SHA256 `a3b07b3cff51f79a5e9ac21f46bea9303ee7032b696428cd3cc70f47c8c4c930` (matches pre-verified hash) |
| `mcp` | `1.26.0`, wheel SHA256 `904a21c33c25aa98ddbeb47273033c435e595bbacfdb177f4bd87f6dceebe1ca` |
| Install result | `Successfully installed` all 30 |

Non-fatal warning at import: `pydantic_settings` emits
`IncompleteFieldDefinitionWarning: Field 'lifespan' ...`. It did not affect the
exercised route.

## 4. Empty-environment isolation (allowlist, not denylist) **[correction]**

The initial `cleanenv.sh` used a **denylist**, so every unlisted variable
(`DISPLAY`, `DBUS_SESSION_BUS_ADDRESS`, the host `XDG_RUNTIME_DIR`, harness IPC,
loader vars, ...) was inherited. That failed the task's explicit
clean-environment condition.

The corrected `cleanenv_run` launches every command with `env -i` and an
explicit fixed allowlist, for the interpreter **and** all subprocesses:

```
S, HOME, XDG_CONFIG_HOME, XDG_CACHE_HOME, XDG_DATA_HOME, XDG_STATE_HOME,
XDG_RUNTIME_DIR, TMPDIR, TMP, TEMP, BLENDER_USER_CONFIG, BLENDER_USER_SCRIPTS,
BLENDER_USER_DATAFILES, BLENDER_USER_EXTENSIONS, PATH, PYTHONNOUSERSITE=1,
PYTHONDONTWRITEBYTECODE=1, LANG=C.UTF-8, LC_ALL=C.UTF-8
```

`PATH` is the fixed `$S/venv/bin:/usr/bin:/bin`; interpreters and the engine are
invoked by absolute path. `HOME`/XDG/temp/Blender-user dirs point inside the
scratch root. `BLENDER_BIN` and `MCP_GUARD_REPORT` are injected only where
needed. `BLENDER_SYSTEM_*` is left unset so Blender uses its **engine-bundled**
system script/asset paths.

**Synthetic-sentinel proof** (`scripts/test_cleanenv.sh`, 39 assertions ok):
the caller exports sentinel values for 27 risky names, then asserts none survive
into the `env -i` child while the allowlist keys are present with scratch values,
and that the interpreter and a grandchild subprocess see the same clean env.
Only names and pass/fail are printed; no real secret values are read or printed.
The only stderr output is the loader's own warning for the synthetic
`LD_PRELOAD` sentinel in the test harness (`out/test_cleanenv_stderr.log`).

This is **process-environment isolation, not a filesystem, network, or security
sandbox**: raw `bpy` runs unsandboxed and can still reach the host.

## 5. Bridge guards (async signatures, fail-closed, contained report) **[correction]**

Upstream `server.py` connects to the add-on at startup
(`blender_lifespan` → `await BlenderConnection.connect()`) and resolves a token
from `$HOME/.blender-mcp/token`. The scratch launcher installs fail-closed
guards **before** `main()` on the exact installed signatures. The correction
made the stubs **async** (`async def connect`, `async def send_command`,
`async def open_connection`) to mirror upstream, preserving the `OSError`-based
fail-closed startup behaviour:

| Guarded symbol | Behaviour |
| --- | --- |
| `async BlenderConnection.connect(self)` | blocked, counted (`BridgeBlockedError`, an `OSError`) |
| `async BlenderConnection.send_command(self, command, params=None)` | blocked, counted |
| `async asyncio.open_connection(host, port, limit=...)` | blocked, counted |
| `default_token_file() -> Path` | blocked, counted |
| `load_auth_token() -> str` | blocked, counted, never returns a value |

**Report-path containment.** `install()` validates the report path with
`Path.resolve()` against the scratch root, refusing relative traversal, absolute
outside paths, a missing scratch root, and symlink escapes, then writes with
`O_NOFOLLOW`/`O_CREAT`/`O_TRUNC` in a validated directory. `install(...)`
without a scratch root and with a report path is fail-closed.

Deterministic guard tests (`scripts/test_bridge_guard.py`, 21 assertions ok)
cover the async `connect`/`send_command` blocks, socket/token blocks, a valid
in-scratch write, and rejection of absolute, `..`, missing-root, and symlink
escape paths. They never contact an editor or a real socket.

Counter evidence after the full smoke (`out/guard_report.json`):

```json
{ "connect_calls": 1, "send_command_calls": 0, "open_connection_calls": 0,
  "load_auth_token_calls": 0, "default_token_file_calls": 0, "token_value_returned": 0 }
```

`connect_calls = 1` is the upstream startup attempt reaching the blocked stub,
which is why the log line `Could not connect to Blender on startup.` appears —
the block is proven, not merely absent. Socket and token guards were never
called, so no add-on socket was opened and no personal token was read. No file
was created under the real `$HOME/.config/blender`.

## 6. Real MCP stdio route **[correction]**

The launcher was spawned through the official MCP SDK stdio transport under
`cleanenv_run`:

| Item | Observed |
| --- | --- |
| Server info | `Blender MCP Server`, version string `1.26.0` (FastMCP reports the SDK version; the package is 0.2.0) |
| Protocol version | `2025-11-25` |
| Tools listed | `27` (includes `blender_python_exec`, headless transport) |
| Tool called | `blender_python_exec`, `transport="headless"`, `script_path` = scratch build script, `timeout_seconds=300` |
| Result | `isError=false`, no tool error, not timed out |

Headless result: Blender `4.5.3 LTS`, object `OGS_SmokeCube` (MESH, 6 polygons,
dimensions `[2.0, 2.0, 2.0]`), material `OGS_SmokeMaterial`, saved `.blend`, and
an explicit GLB export (1,988 bytes).

## 7. Independent verification of the artifacts **[independent]**

The tool response is not trusted for artifact truth. Both checks are separate
read-only processes.

**Reopen in a fresh Blender subprocess** (`--factory-startup`,
`--python-exit-code 1`):

```
VERIFY_BLEND {"object": "OGS_SmokeCube", "dimensions": [2.0, 2.0, 2.0],
              "material": "OGS_SmokeMaterial", "base_color": [0.2, 0.4, 0.8, 1.0],
              "roughness": 0.35, "objects_in_file": ["OGS_SmokeCube"],
              "failures": []}   exit 0
```

The verifier now **asserts** base color and roughness within `1e-4` (previously
they were only reported). Failure of either assertion exits nonzero.

**Independent GLB parse** (stdlib only, no Blender, no MCP package): magic
`glTF`, version `2`, declared length `1988` equals the file size, chunk walk
consumes the file exactly (JSON 1120, BIN 840), 1 node, 1 mesh, 1 material
`OGS_SmokeMaterial`, `baseColorFactor ≈ [0.2, 0.4, 0.8, 1]` asserted within
`1e-4`, attributes `NORMAL/POSITION/TEXCOORD_0`, generator
`Khronos glTF Blender I/O v4.5.48`. The verifier now **decodes the `POSITION`
accessor from the BIN chunk** (24 float3 vertices) instead of trusting JSON
metadata: decoded bounds are `min [-1,-1,-1]` / `max [1,1,1]`, and the accessor
min/max metadata is asserted to agree with the decoded geometry. `failures: []`,
exit 0.

Output hashes (GLB content stable across runs; the `.blend` hash varies because
Blender embeds run-specific bytes):

- `ogs_smoke_cube.glb` `8b64112981780b260658741dae43bf3f1dae2b98c69bbbd255c7875afebbff4d` (same as the historical run)
- `ogs_smoke_cube.blend` `fd8c36f92890e9fbf612b911980b274f93d47461585f64367324f3d49266f361` (correction run; historical run was `6cd77dcc...`)

## 8. Reproducible commands

With `S` pointing at the scratch root (all scripts exist; the harness writes
only inside `$S`; no downloads or installs):

```bash
S=<smoke-root>
export S
source "$S/scripts/cleanenv.sh"

bash "$S/scripts/run_smoke.sh"                       # all checks below, one log

bash "$S/scripts/test_cleanenv.sh"                   # empty-env sentinel test
cleanenv_run "$S/venv/bin/python" "$S/scripts/test_bridge_guard.py"
cleanenv_run env BLENDER_BIN="$S/engine/blender-4.5.3-linux-x64/blender" \
    timeout 420 "$S/venv/bin/python" "$S/scripts/mcp_smoke_client.py"
cat "$S/out/guard_report.json"
cleanenv_run "$S/venv/bin/python" "$S/scripts/verify_archive.py" \
    "$S/download/blender-4.5.3-linux-x64.tar.xz"
```

Artifact locators: `out/run_smoke.log`, `out/test_cleanenv_stderr.log`,
`out/resolution.json`, `out/requirements.lock`, `out/lock-summary.json`,
`out/mcp_smoke_evidence.json`, `out/guard_report.json`,
`out/verify_blend_report.json`, `out/verify_glb_report.json`,
`out/verify_archive_report.json`, `out/ogs_smoke_cube.blend`,
`out/ogs_smoke_cube.glb`.

## 9. Artifact-only independent verification (`env -i`, no provisioning)

For a read-only verifier; none of these generate MCP traffic or provision
anything, and all are `env -i`-isolated:

```bash
S=<smoke-root>
B="$S/engine/blender-4.5.3-linux-x64/blender"

env -i PATH=/usr/bin:/bin "$S/venv/bin/python" \
    "$S/scripts/verify_glb.py" "$S/out/ogs_smoke_cube.glb"
env -i PATH=/usr/bin:/bin "$S/venv/bin/python" \
    "$S/scripts/verify_archive.py" "$S/download/blender-4.5.3-linux-x64.tar.xz"
env -i HOME="$S/home" XDG_CONFIG_HOME="$S/home/.config" \
    XDG_CACHE_HOME="$S/home/.cache" XDG_DATA_HOME="$S/home/.local/share" \
    XDG_STATE_HOME="$S/home/.local/state" XDG_RUNTIME_DIR="$S/home/.run" \
    TMPDIR="$S/tmp" PATH=/usr/bin:/bin \
    "$B" --factory-startup -b "$S/out/ogs_smoke_cube.blend" \
    --python-exit-code 1 --python "$S/scripts/verify_blend.py"
(cd "$S/download" && sha256sum -c --ignore-missing blender-4.5.3.sha256)
sha256sum "$S/out/ogs_smoke_cube.glb"
```

Expected: `VERIFY_GLB_OK`, `VERIFY_ARCHIVE_OK`, `VERIFY_BLEND` with
`"failures": []` and exit 0, the checksum line `OK`, and the GLB hash above.
Historical scratch scripts and outputs were preserved under
`$S/historical/run1-scripts` and `$S/historical/run1-out`.

## 10. Evidence classes **[correction]**

- **Historical** (initial worker): engine/venv/lock figures, the original
  extraction census, the first smoke outputs. Reported here as historical,
  not as independent proof.
- **Correction** (this pass): empty-env allowlist + sentinel proof, async guard
  fix + report-path containment, corrected verifiers, and a fresh full smoke
  (all 9 steps exit 0).
- **Independent** (separate read-only process): the `.blend` reopen, the GLB
  decode, the archive containment stream, and the checksum re-verification.

**Test-first (correction, observed).** The earlier doc's guard RED
(`ModuleNotFoundError: No module named 'bridge_guard'`) is worker-reported and
is **not promoted** here. The correction's own observed lifecycle:

- RED: `test_cleanenv.sh` → `FAIL cleanenv_run_defined missing empty-environment
  runner` (+ `RED denylist still exports DISPLAY to children`), exit 1;
  `test_bridge_guard.py` → `TypeError: install() got an unexpected keyword
  argument 'scratch_root'`, exit 1.
- GREEN: `CLEANENV_TESTS_OK` (39/39), `GUARD_TESTS_OK` (21/21), both exit 0.

No meaningful RED exists for the engine feasibility probe or the headless
execution run; those are reported as functional evidence only.

## 11. Failures, limits and pending work

- **Correction run:** no failures or skips; every step exited 0. Only the
  non-fatal `pydantic_settings` warning and the harness loader sentinel warning
  were emitted.
- **Scope limits.** Only the adapted headless route was exercised. Raw `bpy` is
  unsandboxed; environment isolation is not a security sandbox. The bridge/live
  add-on route, upstream unmodified startup, Pi-native MCP loading
  (`.pi/mcp.json` still not implemented), project-target binding, shared-server
  routing, and the full V1 platform are **not** verified here.
- **Independent verification completed.** Fresh artifact-only checks passed:
  `.blend` reopen (dimensions, color and roughness), GLB binary POSITION decoding
  and material assertions, static archive containment, published checksum and
  artifact/lock hashes. The clean-environment sentinel test passed 39/39.
  No required execution check failed. The optional guard suite was not executed
  independently because it deliberately attempts outside-scratch report paths;
  its 21/21 result remains worker-produced evidence with independent static
  inspection. The independent verifier did not rerun MCP generation or observe
  the historical RED; it verified existing artifacts and fresh environment GREEN.
- **Pending.** Python 3.14 was exercised through resolution and this route only,
  not the whole package surface. The scratch root is ephemeral; if `/tmp` is
  cleared, re-provisioning requires the recorded URLs, sizes and hashes and
  applicable authorization. Broader integration remains outside this evidence.
