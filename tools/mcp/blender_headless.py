"""Minimal source-controlled Blender headless adapter (stdio MCP, runner).

This module is deliberately small and self-contained: a stdlib-only subprocess
runner that executes a trusted Python script inside a locally installed Blender
binary, plus a lazy MCP surface with exactly two tools.

Trust model (read before use):

* The script executed by ``blender_python_exec`` runs arbitrary Python inside
  Blender. It is NOT sandboxed. It can read and write any path the host user can
  reach, regardless of the declared ``output_root``. ``output_root`` only
  supplies the controlled HOME/XDG/tmp environment, not artifact confinement.
* Validation is static and best-effort. It rejects symlinked paths, missing
  files, and script paths outside ``project_root`` at call time; it cannot
  eliminate time-of-check/time-of-use races.
* The wrapper-owned ``output_root/runtime`` tree is preflighted with ``lstat``
  before any directory is created: a symlinked runtime component is a typed
  ``BlenderConfigError``, and an ordinary non-directory component is a structured
  setup failure returned as ``ok=False``. This rejects a static link planted
  before the trusted script runs; it is not race-proof.
* ``args`` must be a JSON object with string keys recursively (JSON itself only
  allows string keys), and the same canonical encoding is used for validation
  and for the engine argv.
* ``subprocess.run`` captures the full child output in memory before it is
  bounded for return. A runaway script can still consume host memory; there is
  no hard memory cap in this slice.

Configuration is fixed per instance and comes from the process environment at
startup (``BLENDER_BIN``, ``OGS_BLENDER_PROJECT_ROOT``, ``OGS_BLENDER_OUTPUT_ROOT``).
There is no implicit engine discovery and no per-call override of the engine or
roots.
"""

from __future__ import annotations

import dataclasses
import json
import os
import stat
import subprocess
import sys
import time

ENV_BLENDER_BIN = "BLENDER_BIN"
ENV_PROJECT_ROOT = "OGS_BLENDER_PROJECT_ROOT"
ENV_OUTPUT_ROOT = "OGS_BLENDER_OUTPUT_ROOT"

DEFAULT_TIMEOUT_MAX_SECONDS = 300
MAX_OUTPUT_BYTES = 16 * 1024
RUNTIME_DIRNAME = "runtime"

_SHELL = False


class BlenderConfigError(ValueError):
    """Raised when engine configuration, a script path, or args are invalid."""


class McpDependencyError(RuntimeError):
    """Raised when the lazy ``mcp`` dependency is unavailable."""


def _require_absolute(value: object, label: str) -> str:
    if not isinstance(value, str) or not value:
        raise BlenderConfigError(f"{label} must be a non-empty string")
    if not os.path.isabs(value):
        raise BlenderConfigError(f"{label} must be an absolute path: {value!r}")
    return os.path.normpath(value)


def _reject_symlink_components(path: str, label: str) -> None:
    current = os.sep
    for part in path.split(os.sep):
        if not part:
            continue
        current = os.path.join(current, part)
        if os.path.islink(current):
            raise BlenderConfigError(f"{label} has a symlinked component: {current}")


def _validate_directory(value: object, label: str) -> str:
    path = _require_absolute(value, label)
    _reject_symlink_components(path, label)
    if not os.path.isdir(path):
        raise BlenderConfigError(f"{label} is not an existing directory: {path}")
    return path


def _validate_executable(value: object, label: str) -> str:
    path = _require_absolute(value, label)
    _reject_symlink_components(path, label)
    if not os.path.isfile(path):
        raise BlenderConfigError(f"{label} is not an existing regular file: {path}")
    if not os.access(path, os.X_OK):
        raise BlenderConfigError(f"{label} is not executable: {path}")
    return path


def _validate_timeout_cap(value: object) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise BlenderConfigError("timeout_max_seconds must be an integer")
    if value <= 0:
        raise BlenderConfigError("timeout_max_seconds must be positive")
    return value


def _validate_args(args: object) -> dict:
    if args is None:
        return {}
    if not isinstance(args, dict):
        raise BlenderConfigError("args must be a dictionary")
    _require_string_keys(args)
    return args


def _require_string_keys(value: object, path: str = "args") -> None:
    """JSON object keys must be strings; reject non-string keys recursively."""
    if isinstance(value, dict):
        for key, child in value.items():
            if not isinstance(key, str):
                raise BlenderConfigError(
                    f"{path} object keys must be strings (got {type(key).__name__})"
                )
            _require_string_keys(child, f"{path}.{key}")
    elif isinstance(value, (list, tuple)):
        for index, child in enumerate(value):
            _require_string_keys(child, f"{path}[{index}]")


def _encode_args(args: dict) -> str:
    """Single canonical JSON encoding, used for both validation and the argv."""
    try:
        return json.dumps(args, sort_keys=True, allow_nan=False)
    except (TypeError, ValueError) as exc:
        raise BlenderConfigError(f"args must be JSON-serializable: {exc}") from exc


def _bound(data: bytes | None) -> tuple[str, bool]:
    if not data:
        return "", False
    truncated = len(data) > MAX_OUTPUT_BYTES
    # Keep the tail: engine failures print their useful diagnostics last.
    text = data[-MAX_OUTPUT_BYTES:].decode("utf-8", "replace")
    return text, truncated


@dataclasses.dataclass(frozen=True)
class HeadlessConfig:
    """Frozen, per-instance Blender binding. Validated at construction time."""

    executable: str
    project_root: str
    output_root: str
    timeout_max_seconds: int = DEFAULT_TIMEOUT_MAX_SECONDS

    def __post_init__(self) -> None:
        cap = _validate_timeout_cap(self.timeout_max_seconds)
        executable = _validate_executable(self.executable, "executable")
        project_root = _validate_directory(self.project_root, "project_root")
        output_root = _validate_directory(self.output_root, "output_root")
        object.__setattr__(self, "timeout_max_seconds", cap)
        object.__setattr__(self, "executable", executable)
        object.__setattr__(self, "project_root", project_root)
        object.__setattr__(self, "output_root", output_root)

    @classmethod
    def from_env(cls, env: dict | None = None) -> "HeadlessConfig":
        source = os.environ if env is None else env
        required = (ENV_BLENDER_BIN, ENV_PROJECT_ROOT, ENV_OUTPUT_ROOT)
        missing = [name for name in required if not source.get(name)]
        if missing:
            raise BlenderConfigError(
                "missing required environment: " + ", ".join(missing)
            )
        return cls(
            executable=source[ENV_BLENDER_BIN],
            project_root=source[ENV_PROJECT_ROOT],
            output_root=source[ENV_OUTPUT_ROOT],
        )

    def validate_script(self, script_path: object) -> str:
        path = _require_absolute(script_path, "script_path")
        _reject_symlink_components(path, "script_path")
        if not os.path.commonpath([path, self.project_root]) == self.project_root:
            raise BlenderConfigError(
                f"script_path escapes project_root ({self.project_root}): {path}"
            )
        if not os.path.isfile(path):
            raise BlenderConfigError(f"script_path is not an existing regular file: {path}")
        return path

    def resolve_timeout(self, timeout_seconds: object) -> int:
        if timeout_seconds is None:
            return self.timeout_max_seconds
        if isinstance(timeout_seconds, bool) or not isinstance(timeout_seconds, int):
            raise BlenderConfigError("timeout_seconds must be an integer")
        if timeout_seconds <= 0:
            raise BlenderConfigError("timeout_seconds must be positive")
        if timeout_seconds > self.timeout_max_seconds:
            raise BlenderConfigError(
                f"timeout_seconds {timeout_seconds} exceeds the configured cap "
                f"{self.timeout_max_seconds}"
            )
        return timeout_seconds

    def runtime_dirs(self) -> dict:
        home = os.path.join(self.output_root, RUNTIME_DIRNAME, "home")
        return {
            "home": home,
            "tmp": os.path.join(self.output_root, RUNTIME_DIRNAME, "tmp"),
            "config": os.path.join(home, ".config"),
            "cache": os.path.join(home, ".cache"),
            "data": os.path.join(home, ".local", "share"),
            "state": os.path.join(home, ".local", "state"),
            "run": os.path.join(home, ".run"),
        }

    def _runtime_target_dirs(self, dirs: dict) -> list:
        return list(dirs.values()) + [
            os.path.join(dirs["config"], "blender", "scripts"),
            os.path.join(dirs["data"], "blender", "extensions"),
        ]

    def _preflight_runtime_dirs(self, targets: list) -> None:
        """Reject symlinked/non-directory existing runtime components before any write.

        Uses lstat on every existing component of every target so a static symlink
        cannot redirect runtime creation outside output_root. Raises
        BlenderConfigError for symlink or containment violations (typed validation)
        and NotADirectoryError for an ordinary non-directory component (which
        _run converts to a structured setup failure). This is static validation,
        not race-proof.
        """
        root = os.path.realpath(self.output_root)
        for target in targets:
            relative = os.path.relpath(target, self.output_root)
            current = self.output_root
            for part in relative.split(os.sep):
                current = os.path.join(current, part)
                try:
                    info = os.lstat(current)
                except FileNotFoundError:
                    break
                if stat.S_ISLNK(info.st_mode):
                    raise BlenderConfigError(
                        f"runtime path has a symlinked component: {current}"
                    )
                if not stat.S_ISDIR(info.st_mode):
                    raise NotADirectoryError(
                        f"runtime path component is not a directory: {current}"
                    )
                resolved = os.path.realpath(current)
                if os.path.commonpath([resolved, root]) != root:
                    raise BlenderConfigError(
                        f"runtime path escapes output_root: {current}"
                    )

    def _ensure_runtime_dirs(self) -> dict:
        dirs = self.runtime_dirs()
        targets = self._runtime_target_dirs(dirs)
        # Preflight the entire list before creating any earlier path, so a bad
        # component cannot leave partial sibling directories behind.
        self._preflight_runtime_dirs(targets)
        for path in targets:
            os.makedirs(path, exist_ok=True)
        return dirs

    def _build_env(self, dirs: dict) -> dict:
        return {
            "HOME": dirs["home"],
            "XDG_CONFIG_HOME": dirs["config"],
            "XDG_CACHE_HOME": dirs["cache"],
            "XDG_DATA_HOME": dirs["data"],
            "XDG_STATE_HOME": dirs["state"],
            "XDG_RUNTIME_DIR": dirs["run"],
            "TMPDIR": dirs["tmp"],
            "TMP": dirs["tmp"],
            "TEMP": dirs["tmp"],
            "BLENDER_USER_CONFIG": os.path.join(dirs["config"], "blender"),
            "BLENDER_USER_SCRIPTS": os.path.join(dirs["config"], "blender", "scripts"),
            "BLENDER_USER_DATAFILES": os.path.join(dirs["data"], "blender"),
            "BLENDER_USER_EXTENSIONS": os.path.join(dirs["data"], "blender", "extensions"),
            "PATH": os.path.dirname(self.executable) + ":/usr/bin:/bin",
            "PYTHONNOUSERSITE": "1",
            "PYTHONDONTWRITEBYTECODE": "1",
            "LANG": "C.UTF-8",
            "LC_ALL": "C.UTF-8",
        }

    def _run(self, argv: list, timeout_seconds: int) -> tuple:
        started = time.monotonic()
        try:
            dirs = self._ensure_runtime_dirs()
        except OSError as exc:
            # Ordinary preparation failure (existing file, permission, ...) is a
            # structured outcome, not a leaked filesystem exception. Symlink and
            # containment violations raise BlenderConfigError and propagate.
            return BlenderRunResult(
                ok=False,
                exit_code=None,
                stdout="",
                stderr="",
                stdout_truncated=False,
                stderr_truncated=False,
                timed_out=False,
                duration_seconds=time.monotonic() - started,
                error=f"cannot prepare runtime directories: {exc}",
            )
        try:
            completed = subprocess.run(
                argv,
                cwd=self.project_root,
                env=self._build_env(dirs),
                capture_output=True,
                timeout=timeout_seconds,
                check=False,
                shell=_SHELL,
            )
        except subprocess.TimeoutExpired as exc:
            duration = time.monotonic() - started
            stdout, stdout_truncated = _bound(exc.stdout)
            stderr, stderr_truncated = _bound(exc.stderr)
            return BlenderRunResult(
                ok=False,
                exit_code=None,
                stdout=stdout,
                stderr=stderr,
                stdout_truncated=stdout_truncated,
                stderr_truncated=stderr_truncated,
                timed_out=True,
                duration_seconds=duration,
                error=f"timeout after {timeout_seconds}s",
            )
        except OSError as exc:
            duration = time.monotonic() - started
            return BlenderRunResult(
                ok=False,
                exit_code=None,
                stdout="",
                stderr="",
                stdout_truncated=False,
                stderr_truncated=False,
                timed_out=False,
                duration_seconds=duration,
                error=f"cannot execute engine: {exc}",
            )
        duration = time.monotonic() - started
        stdout, stdout_truncated = _bound(completed.stdout)
        stderr, stderr_truncated = _bound(completed.stderr)
        return BlenderRunResult(
            ok=completed.returncode == 0,
            exit_code=completed.returncode,
            stdout=stdout,
            stderr=stderr,
            stdout_truncated=stdout_truncated,
            stderr_truncated=stderr_truncated,
            timed_out=False,
            duration_seconds=duration,
            error=None if completed.returncode == 0 else f"engine exited {completed.returncode}",
        )

    def run_script(self, script_path: object, args: object = None,
                   timeout_seconds: object = None) -> "BlenderRunResult":
        """Run a validated project script. Raises before any subprocess on bad input."""
        script = self.validate_script(script_path)
        payload = _validate_args(args)
        timeout = self.resolve_timeout(timeout_seconds)
        encoded = _encode_args(payload)
        argv = [
            self.executable,
            "--factory-startup",
            "-b",
            "--python-exit-code",
            "1",
            "--python",
            script,
            "--",
            encoded,
        ]
        return self._run(argv, timeout)

    def check_health(self) -> dict:
        """Run ``blender --version`` with the controlled environment. No addon contact."""
        result = self._run([self.executable, "--version"], self.timeout_max_seconds)
        first_line = result.stdout.strip().splitlines()[:1]
        return {
            "ok": result.ok,
            "executable": self.executable,
            "version": first_line[0] if first_line else None,
            "exit_code": result.exit_code,
            "timed_out": result.timed_out,
            "duration_seconds": result.duration_seconds,
            "error": result.error,
        }


@dataclasses.dataclass(frozen=True)
class BlenderRunResult:
    """Structured outcome of one engine invocation. Bounded stdout/stderr tails."""

    ok: bool
    exit_code: int | None
    stdout: str
    stderr: str
    stdout_truncated: bool
    stderr_truncated: bool
    timed_out: bool
    duration_seconds: float
    error: str | None = None

    def to_dict(self) -> dict:
        return dataclasses.asdict(self)


TOOL_EXEC_DESCRIPTION = (
    "Run a trusted Python script inside the configured local Blender in headless "
    "factory-startup mode and return exit code, bounded stdout/stderr tails, "
    "truncation flags, and timeout status. The script runs arbitrary Python with "
    "the host user's filesystem permissions; this is NOT a sandbox and output_root "
    "does NOT confine what the script may read or write."
)
TOOL_HEALTH_DESCRIPTION = (
    "Report the configured Blender executable and its `--version` output using the "
    "controlled runtime environment. Contacts no add-on, editor, or network service."
)


def create_server(config: HeadlessConfig):
    """Build the two-tool FastMCP app. Imports ``mcp`` lazily; stdio transport only."""
    try:
        from mcp.server.fastmcp import FastMCP
    except ImportError as exc:  # pragma: no cover - exercised via subprocess test
        raise McpDependencyError(
            "the 'mcp' package is required for the MCP surface; install it in the "
            "adapter environment (this runner itself needs no MCP dependency)"
        ) from exc

    server = FastMCP(
        name="ogs-blender-headless",
        instructions="Blender headless execution tools for OGS. Trusted arbitrary Python, not sandboxed.",
        lifespan=None,
    )

    @server.tool(name="blender_python_exec", description=TOOL_EXEC_DESCRIPTION)
    def blender_python_exec(script_path: str, args: dict | None = None,
                            timeout_seconds: int | None = None) -> dict:
        return config.run_script(script_path, args, timeout_seconds).to_dict()

    @server.tool(name="health", description=TOOL_HEALTH_DESCRIPTION)
    def health() -> dict:
        return config.check_health()

    return server


def main(argv: list | None = None) -> int:
    """Validate configuration, then run the stdio MCP server. No engine probe here."""
    del argv  # stdio only; no CLI flags are accepted
    try:
        config = HeadlessConfig.from_env()
    except BlenderConfigError as exc:
        print(f"blender_headless: configuration error: {exc}", file=sys.stderr)
        return 2
    try:
        server = create_server(config)
    except McpDependencyError as exc:
        print(f"blender_headless: {exc}", file=sys.stderr)
        return 3
    server.run()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
