# Blender headless adapter (Python)

`tools/mcp/blender_headless.py` is a small, self-contained adapter that runs a
trusted Python script inside a local Blender binary in headless mode and exposes
it over stdio MCP with exactly two tools. It exists so OGS can drive a real
engine through Pi-native MCP later without depending on an upstream Blender MCP
server or its add-on bridge.

This is a **source slice**: it is validated with a fake engine in temp
directories. It does not provision Blender, install packages, contact an editor
add-on, or configure Pi.

## Quick path

1. Install a local Blender binary you trust.
2. Point the adapter at it with absolute paths and start the server:
   ```bash
   export BLENDER_BIN=/absolute/path/to/blender
   export OGS_BLENDER_PROJECT_ROOT=/absolute/path/to/project
   export OGS_BLENDER_OUTPUT_ROOT=/absolute/path/to/scratch-output
   python3 tools/mcp/blender_headless.py        # stdio MCP, no CLI flags
   ```
   Missing or invalid configuration prints one actionable line to stderr and
   exits non-zero; no engine subprocess starts.
3. Run the tests (see [Verification](#verification)).

## Configuration

| Variable | Meaning |
|----------|---------|
| `BLENDER_BIN` | Absolute path to an existing, executable Blender **regular** file. |
| `OGS_BLENDER_PROJECT_ROOT` | Absolute existing directory; scripts must live inside it and the engine runs with this as `cwd`. |
| `OGS_BLENDER_OUTPUT_ROOT` | Absolute existing directory; owns the `runtime/` HOME/XDG/tmp tree. |

Rules the adapter enforces before any subprocess:

- No implicit `PATH`/environment engine discovery; roots and executable come only
  from these variables and are fixed for the process lifetime.
- Rejects symlinked roots, symlinked path components, and symlinked file inputs.
- Rejects relative, missing, non-regular, or escaping `script_path` values.
- `args` must be a JSON object with string keys at every level (JSON only allows
  string keys); one canonical encoding is used for both validation and the argv.
- Preflights the wrapper-owned `output_root/runtime` tree with `lstat` before
  creating anything. A symlinked runtime component is a typed error; an ordinary
  non-directory component or other preparation failure is a structured `ok=False`
  result. No engine subprocess starts in either case.
- `timeout_seconds` must be a positive integer (bools are invalid) within the
  configured cap (default 300s).

## Tool surface

| Tool | Purpose |
|------|---------|
| `blender_python_exec(script_path, args, timeout_seconds)` | Run a validated project script and return `ok`, `exit_code`, bounded `stdout`/`stderr` tails, truncation flags, `timed_out`, and `error`. |
| `health()` | Run `blender --version` with the controlled environment. No add-on or network contact. |

The engine is launched without a shell:
`blender --factory-startup -b --python-exit-code 1 --python <script> -- <json-args>`.
The child environment is a fixed allowlist (scratch HOME/XDG/tmp, fixed locale,
`PYTHONNOUSERSITE`, `PYTHONDONTWRITEBYTECODE`); GUI, proxy, credential, loader,
`PYTHONPATH`, and harness variables are not inherited.

## Trust model and limits

Read this before treating the adapter as a boundary:

- **Not a sandbox.** The executed script is trusted arbitrary Python. It runs with
  the host user's permissions and can read or write any path, including paths
  outside `output_root`. `output_root` only supplies HOME/XDG/tmp, not artifact
  confinement.
- **Static validation only.** Path checks run at call time and cannot eliminate
  time-of-check/time-of-use races. The runtime preflight rejects a symlink that
  already exists when the call starts; it does not make concurrent filesystem
  races impossible.
- **Memory is not capped.** `subprocess.run` buffers the full child output in
  memory before the returned tails are truncated to 16 KiB per stream. A runaway
  script can still consume host memory.
- **No caller override.** Tool arguments cannot change the executable, roots, or
  environment; only the script path, JSON args, and an in-cap timeout are
  caller-controlled.

## Verification

From the repository root:

```bash
# 1. stdlib run: engine/runner tests pass; MCP-surface tests skip when 'mcp' is absent.
python3 -B -m unittest discover -s tests/python -v

# 2. run again with a Python that has 'mcp' installed to assert the real 2-tool surface.
<python-with-mcp> -B -m unittest discover -s tests/python -v
```

Both runs use only a fake executable engine written into unittest temp
 directories, so the unit suite needs no real Blender, network, or MCP transport.
Use `-B` so the tests do not write bytecode.

## Real runtime evidence (source adapter)

This adapter was also exercised once as a real stdio MCP server against a locally
available Blender 4.5.3 LTS engine and the pinned `mcp` 1.26.0 SDK, inside an
isolated scratch tree (separate project/output/HOME/XDG/tmp dirs). Observed:

- `initialize` + `list-tools` returned exactly `blender_python_exec` and `health`.
- `health()` returned `ok: true` with `version: "Blender 4.5.3 LTS"` via `--version`.
- `blender_python_exec()` returned `ok: true`, `exit_code: 0`, `timed_out: false`,
  `error: null`, and bounded untruncated output; the trusted script created the
  named cube/material, dimensions `[2, 2, 2]`, base color `[0.2, 0.4, 0.8, 1.0]`,
  roughness `0.35`, and saved a `.blend` plus an explicit GLB under `output_root`.
- Independent checks passed: a fresh Blender process reopened the `.blend` and
  confirmed object/material/dimensions/color/roughness, and a stdlib-only GLB
  decode confirmed container length, one mesh/material, and POSITION bounds.
- SDK response shape: tools returned `TextContent` JSON with `structuredContent`
  `null` in this `mcp` version; the JSON text is the contract.

Limitations: this is source-adapter runtime evidence only. It is not Pi-loaded
configuration, persistent provisioning, or shared-routing acceptance, and it does
not replace the deterministic unit suite. The engine ran with the adapter's
allowlist environment (its Blender config/cache landed under
`output_root/runtime`), but the executed script is still trusted arbitrary Python
and is not sandboxed; script artifacts are not confined to `output_root`.

## Checklist

- [ ] Engine and roots are supplied as absolute, non-symlinked paths.
- [ ] `BLENDER_BIN` is a real executable, not a wrapper that changes the argv contract.
- [ ] Callers understand scripts are unsandboxed and `output_root` is not confinement.
- [ ] Both verification commands were run; the MCP-surface tests either passed or skipped honestly.

## Next step

MCP-3b: provision reproducible launcher paths and create-only Pi configuration,
then MCP-3c: an explicitly authorized Pi-loaded functional smoke. Those steps are
out of scope for this source slice.
