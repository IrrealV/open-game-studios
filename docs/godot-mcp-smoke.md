# Godot MCP scene smoke — isolated real stdio execution evidence

Bounded runtime evidence for the MCP execution-validation task (MCP-2). The
published `@coding-solo/godot-mcp@0.1.1` server was driven over **real MCP
stdio** against a disposable copy of the trusted 2D fixture, and the produced
`.tscn` was independently reloaded by a fresh headless Godot 4 process. This is
**not** Pi-loaded integration, **not** editor/listener/shared-server routing,
**not** a V1-03 recertification, and **not** full V1 acceptance.

- Historical run: 2026-10-04 (host clock), HEAD
  `e971fa35666870abd992cb7cb6afb860e9acc7e2`; public record in `ae52d0c`.
- Scratch root: `<smoke-root>` (ephemeral; not shipped in a clone).
- Evidence classes: **[observed]** = historical SDK run; **[guard]** = its
  deterministic harness test. Later native Pi health is separate:
  [current MCP checkpoint](mcp-config.md#evidence-checkpoint).

## 0. Repo state **[observed]**

`git status --porcelain` before and after: `?? game-studio` (pre-existing,
untouched) plus this new untracked `docs/godot-mcp-smoke.md`. `HEAD` stayed
`e971fa3`. The original fixture files were byte-identical before and after:

| fixture file | sha256 |
| --- | --- |
| `testdata/godot-minimal-2d/player.gd` | `97a75442b983056a01b26812bfd9dea72db1480eff19e53006369da40dc92f39` |
| `testdata/godot-minimal-2d/player.tscn` | `6acf6857355b7473f815399bbf0af1066b27362a05b5be40959c27f0093b85f3` |
| `testdata/godot-minimal-2d/project.godot` | `d8c0cc7b9427ef6e17da67717a9ce4865e23bfbd57c3db7fb587b79705dce2c6` |
| `testdata/godot-minimal-2d/verify_mechanic.gd` | `d0a49dc454dcd2265e81255d6e07327320178d5fc6b68aaf324262c419cc118f` |

## 1. Host and engine preflight **[observed]**

`Godot_v4.7.2-stable_linux.x86_64` at `$TOOLS/godot/4.7.2/` — mode `0700`,
size `146414384` (matches the approved candidate). `--version` under the
allowlist → `4.7.2.stable.official.ed1daf0bf`, exit 0. Node `v26.8.2` and an
explicit provisioned npm CLI were used; no normal-PATH discovery is implied.

## 2. Package provenance and integrity **[observed]**

| Item | Observed |
| --- | --- |
| Tarball | `https://registry.npmjs.org/@coding-solo/godot-mcp/-/godot-mcp-0.1.1.tgz` |
| Size / SHA1 | `23688` bytes / `1a52ba94095fef4ebb51916503989f8dcb3cfc94` (published) |
| sha512 integrity | `tSwmKN5D3JWGfnyPFxijxjdsT+GKIQrf9g6zYNefLvOVYx2UTC9PhyFFHc9vphtjyFrtjYf1n5Vq1zRNN3UXBA==` (published) |
| SHA256 | `980edf926c3591093f84ed2c399ef7ce7a3210356eff799f70e52b36c789350d` |
| Installed entrypoint | `build/index.js` sha256 `fd3f5df677988285606283bf29eb7ca006d2ed89faf75870091c890638806dc0` — identical to the archive member, never rebuilt |

Static archive census (no extraction): 5 members / 5 files / 0 dirs / 0 links;
0 absolute names, 0 `..` components, 0 escaping links, 0 device/other members;
`package/build/index.js` present. Download used ordinary HTTPS with no
credentials and no inherited proxy.

## 3. Dependency lock review **[observed]**

Resolved a consumer lock with `npm install --package-lock-only
--ignore-scripts`; then installed with `npm ci --ignore-scripts` (lifecycle
scripts disabled for the package's `prepare`/`build` and every transitive
entry). `lockfileVersion 3`, **46 packages**, **0** entries with
`hasInstallScript`, every entry carrying its registry `integrity`. Lock sha256
`e2cb3ecb03f93b7cb78ff64227570cee597bb285377c71a56bb40470b4baa624`.
The consumer lock root dependency is `@coding-solo/godot-mcp: 0.1.1`.
The package's dependencies include `@modelcontextprotocol/sdk 0.6.0`,
`axios ^1.7.9` (resolved `1.20.0`), and `fs-extra ^11.2.0` (`11.4.1`).

## 4. Empty-environment isolation (allowlist) **[guard]**

Every command — node, npm, the MCP server, the engine and their children — runs
through `scripts/cleanenv.sh` with `env -i` and a fixed allowlist
(`S`, `HOME`, `XDG_*`, `TMPDIR`, `PATH`, `LANG`/`LC_ALL`, and scratch-only
`npm_config_userconfig`/`globalconfig`/`cache`/`registry`). Not inherited:
`DISPLAY`, DBUS, host `XDG_RUNTIME_DIR`, `WSL_*`, `LD_*`, `NODE_OPTIONS`,
`PYTHON*`, `PIP_*`, proxies, credentials, harness IPC, personal npm config.

Synthetic-sentinel test (`scripts/test_cleanenv.sh`):
RED against a denylist runner → `CLEANENV_TESTS 14/53`, exit 1 (the inherited
`NODE_OPTIONS` even broke the child, and no allowlist value was set). GREEN with
the allowlist runner → `CLEANENV_TESTS 53/53`, exit 0; 14 risky names absent in
both the child and a grandchild, allowlist keys present with scratch values.
This is process-environment isolation, **not** a filesystem or security sandbox.

## 5. Safe project and evidence selection **[guard]**

`scripts/project_guard.sh` fails closed on: a non-directory, a symlinked root,
a path resolving outside the scratch root, a missing `project.godot`, any
symlink inside the project, and the engine's own `godot_mcp_test_write.tmp` /
`test_write_access.tmp`. `guard_evidence_path` requires an absolute,
in-scratch, non-symlink parent. Test-first: RED `guard module missing` (exit 1)
→ GREEN `PROJECT_GUARD_TESTS 12/12` (exit 0), including an outside-sentinel
project that stayed byte-unchanged.

The fixed project `P = $S/project` is a `cp -a` copy of the fixture: no
symlinks, no forbidden temp artifacts, `project.godot` present (guard exit 0).

## 6. Real MCP stdio route **[observed]**

One dedicated server process spawned per smoke over the official SDK stdio
transport with an explicit `GODOT_PATH` (no cwd routing; `projectPath` explicit
on every call).

| Item | Observed |
| --- | --- |
| Protocol version | `2024-11-05` |
| Server info | `godot-mcp` `0.1.0`; capabilities `{"tools":{}}` |
| Startup | `GODOT_PATH` validated via `--version`; `Godot MCP server running on stdio` |
| Tools listed | 14 (`… get_project_info, create_scene, add_node …`) |
| Schemas | `create_scene` requires `projectPath, scenePath`; `add_node` requires `projectPath, scenePath, nodeType, nodeName`, accepts `parentNodePath`, does **not** accept `rootNodeType` |

Calls and results:

| Call | Arguments | Result |
| --- | --- | --- |
| `get_project_info` | `{projectPath: P}` | `isError false`; `godotVersion 4.7.2…`, preserved response structure `{scenes:2, scripts:2, assets:0, other:1}` |
| `create_scene` | `{projectPath: P, scenePath: "mcp_smoke.tscn", rootNodeType: "Node3D"}` | `isError false`; file created |
| `add_node` | `{projectPath: P, scenePath: "mcp_smoke.tscn", parentNodePath: "root", nodeType: "Node3D", nodeName: "MCPProof"}` | `isError false`; file updated |

**Package defect found.** `get_project_info` logged
`Error reading project file: ReferenceError: require is not defined` (ESM build
calls `require`), so the returned `name` fell back to `"project"` instead of
`"OGS Minimal Movement"`. The tool still reported `isError false`. Server
success is **not** authoritative: the JS layer only treats stderr containing
`Failed to` as an error and otherwise swallows engine exit codes.

## 7. Independent artifact verification **[observed]**

The tool responses were not trusted. A **fresh** headless Godot process (not the
MCP server) loaded `res://mcp_smoke.tscn` and instantiated it:

```
VERIFY_SCENE {"resource_type":"PackedScene","root_name":"root","root_type":"Node3D",
 "child_name":"MCPProof","child_type":"Node3D","child_owner":"root","failures":[]}
exit 0
```

So the `PackedScene` root is a `Node3D` named `root`, its child `MCPProof` is a
`Node3D`, and the child's owner is the root node. The artifact
`$S/project/mcp_smoke.tscn` was 145 bytes, sha256
`b4446aecbb95a3c4554918e58284968c1d5b689499b32848ecfcf607a037fce4`. The
`unique_id` Godot stamps into the `.tscn` changes per run, so the artifact hash
is not reproducible across runs while its structure is.

## 8. Effects, temp files and limitations **[observed]**

Because `GODOT_DEBUG_MODE` is hard-coded `true`, `create_scene` writes and then
removes `godot_mcp_test_write.tmp` and `test_write_access.tmp` at the project
root. After the run: 0 leftover `*.tmp` files, and only `mcp_smoke.tscn` was
added; the four copied fixture files were byte-identical to source, confirmed
by fresh independent hashes. No unexpected repository mutation was observed.
Writes were configured for scratch paths, but absence of all outside-scratch
writes was not established by filesystem auditing or confinement.

Limitations of this historical SDK run: no Pi-native `.pi/mcp.json` loading,
no editor/`run_project`/
`stop_project`/listener route, no shared server, no `3D` verifier change and no
V1-03 recertification. The environment allowlist does not sandbox the
filesystem or network. The `get_project_info` `require` defect above and the
non-deterministic `unique_id` are reported, not worked around.

## 9. Reproducible and artifact-only commands

Full route (`scripts/run_smoke.sh`) requires provisioning and network.
The worker's aggregate `verify_artifacts.sh` is **not strictly read-only**:
its sourced `cleanenv.sh` creates scratch directories and truncates the scratch
npm configuration files. The independent verifier did not execute that wrapper
or either guard suite; it used explicit empty-environment commands instead.
No independent `VERIFY_ARTIFACTS_OK` result is claimed.

Artifact-only checks, without provisioning, network or MCP generation:

```bash
S=<smoke-root>
G="$TOOLS/godot/4.7.2/Godot_v4.7.2-stable_linux.x86_64"
env -i S="$S" HOME="$S/home" \
  XDG_CONFIG_HOME="$S/home/.config" XDG_CACHE_HOME="$S/home/.cache" \
  XDG_DATA_HOME="$S/home/.local/share" XDG_STATE_HOME="$S/home/.local/state" \
  XDG_RUNTIME_DIR="$S/home/.run" TMPDIR="$S/tmp" TMP="$S/tmp" TEMP="$S/tmp" \
  PATH="$S/node/bin:/usr/bin:/bin" LANG=C.UTF-8 LC_ALL=C.UTF-8 \
  timeout 60 "$G" --headless --path "$S/project" --script "$S/scripts/verify_scene.gd"
env -i PATH=/usr/bin:/bin /bin/bash "$S/scripts/verify_archive.sh" \
  "$S/download/godot-mcp-0.1.1.tgz"
env -i PATH=/usr/bin:/bin "$S/node/bin/node" "$S/scripts/review_lock.mjs" \
  "$S/work/npm/package-lock.json"
```

These commands passed independently (exit 0), including fresh scene
`failures: []`. The independent verifier also checked engine version,
tarball SHA1/SHA512, archive-member versus installed build hash, lock and scene
hashes, and all four source/copy fixture hashes. All matched the recorded
values. This confirms artifacts and provenance; real MCP transport and guard
RED/GREEN remain worker-recorded evidence inspected independently, not a fresh
independent MCP rerun. Engine-managed scratch caches remain possible.

Key artifacts: `out/mcp_smoke_evidence.json`, `out/mcp_smoke_client.{stdout,stderr}.log`,
`out/lock-review.txt`, `out/verify_scene.{stdout,stderr}.log`,
`download/godot-mcp-0.1.1.tgz`, `work/npm/package-lock.json`,
`project/mcp_smoke.tscn`.

## 10. Failures, skips and pending

- **No required check failed.** Every step exited 0 on the final route.
- **Test-first.** Deterministic guards only: cleanenv RED 14/53 → GREEN 53/53;
  project guard RED (missing module) → GREEN 12/12. The real MCP feasibility
  probe has no meaningful prior RED and is reported as functional evidence.
- **Package defect (reported, not patched):** `get_project_info` ESM `require`
  failure; no `name` fidelity from `project.godot`.
- **Not covered by this SDK scene run:** Pi-loaded configuration,
  editor/listener/shared routing, 3D verifier fix, gameplay mechanics, broader
  V1 certification. Later human-reported native Pi health does not establish
  native Pi scene mutation; see the [MCP checkpoint](mcp-config.md#evidence-checkpoint).
  Broader integration remains outside this evidence; the scratch root is ephemeral.
