"""Minimal stdlib-only launcher for OGS persistent MCP runtimes (Blender/Godot).

The launcher takes one explicit mode and absolute paths on the command line and
then ``execve``s the third-party runtime exactly once, so the managed MCP server
replaces this process and inherits the already-open stdio protocol pipes. It
never spawns a child and never parses a config file, descriptor, engine
discovery, download, or default HOME path; there is no ``PATH`` lookup for the
runtime and no shell string.

    blender: execve <interpreter> -B <entrypoint>
             env BLENDER_BIN/OGS_BLENDER_PROJECT_ROOT/OGS_BLENDER_OUTPUT_ROOT
    godot:   execve <interpreter> <entrypoint>   (cwd = project_root)
             env GODOT_PATH

Trust model (read before use):

* Validation is static and best-effort. It rejects relative/missing/non-regular/
  non-executable interpreter and engine paths, missing entrypoints, missing
  project/runtime/output roots, missing Godot ``project.godot`` markers, and any
  existing symlinked path component. It cannot eliminate time-of-check/time-of-use
  races and it is NOT a security sandbox.
* The launcher never creates, truncates, or overwrites anything. Every runtime
  HOME/XDG/tmp directory must already be provisioned by the caller; a missing or
  symlinked component is a typed error before any exec.
* The child environment is a literal allowlist built from scratch. Credentials,
  proxy, GUI, loader, ``PYTHONPATH``, ``NODE_OPTIONS``, and harness variables are
  never inherited, and the whole process environment is ignored.
* The launcher itself must be started by Pi with a clean environment
  (``/usr/bin/env -i PATH=/usr/bin:/bin <python> -B <launcher> ...``); it cannot
  undo dynamic-loader or interpreter settings that the initial bootstrap already
  injected. Stdout is reserved for the MCP protocol: the launcher prints only to
  stderr and exits 2 on validation or exec failure.
* A generic launcher must not blanket-reject ``/tmp`` because legitimate unit
  fixtures and ephemeral runtimes live there. Production provisioning/dispatch
  must refuse scratch or ``/tmp`` references; that policy lives in the deployment
  configuration, not here.
* This launcher exposes no read-only health probe and performs no engine probe.
"""

from __future__ import annotations

import argparse
import dataclasses
import os
import sys

MODES = ("blender", "godot")
RUNTIME_SUBDIRS = ("home", "tmp", "config", "cache", "data", "state", "run")


class LauncherError(ValueError):
    """Raised when a mode, a path, or a root fails validation before exec."""


def _require_absolute(value: object, label: str) -> str:
    if not isinstance(value, str) or not value:
        raise LauncherError(f"{label} must be a non-empty string")
    if not os.path.isabs(value):
        raise LauncherError(f"{label} must be an absolute path: {value!r}")
    return os.path.normpath(value)


def _reject_symlink_components(path: str, label: str) -> None:
    current = os.sep
    for part in path.split(os.sep):
        if not part:
            continue
        current = os.path.join(current, part)
        if os.path.islink(current):
            raise LauncherError(f"{label} has a symlinked component: {current}")


def _validate_executable(value: object, label: str) -> str:
    path = _require_absolute(value, label)
    _reject_symlink_components(path, label)
    if not os.path.isfile(path):
        raise LauncherError(f"{label} is not an existing regular file: {path}")
    if not os.access(path, os.X_OK):
        raise LauncherError(f"{label} is not executable: {path}")
    return path


def _validate_file(value: object, label: str) -> str:
    path = _require_absolute(value, label)
    _reject_symlink_components(path, label)
    if not os.path.isfile(path):
        raise LauncherError(f"{label} is not an existing regular file: {path}")
    return path


def _validate_directory(value: object, label: str) -> str:
    path = _require_absolute(value, label)
    _reject_symlink_components(path, label)
    if not os.path.isdir(path):
        raise LauncherError(f"{label} is not an existing directory: {path}")
    return path


@dataclasses.dataclass(frozen=True)
class LaunchPlan:
    """Frozen, validated launch contract. Construction fails before any exec.

    ``output_root`` is Blender-only and must be ``None`` for Godot mode.
    """

    mode: str
    interpreter: str
    entrypoint: str
    engine: str
    project_root: str
    runtime_root: str
    output_root: str | None = None

    def __post_init__(self) -> None:
        if self.mode not in MODES:
            raise LauncherError(
                f"unknown mode {self.mode!r}; expected one of {', '.join(MODES)}"
            )
        interpreter = _validate_executable(self.interpreter, "interpreter")
        entrypoint = _validate_file(self.entrypoint, "entrypoint")
        engine = _validate_executable(self.engine, "engine")
        project_root = _validate_directory(self.project_root, "project_root")
        runtime_root = _validate_directory(self.runtime_root, "runtime_root")
        # Validate the entire runtime list the process will use, so a missing or
        # symlinked component fails here instead of after the runtime starts.
        for name in RUNTIME_SUBDIRS:
            _validate_directory(
                os.path.join(runtime_root, name), f"runtime '{name}' directory"
            )
        output_root = None
        if self.mode == "blender":
            if self.output_root is None:
                raise LauncherError("blender mode requires --output-root")
            output_root = _validate_directory(self.output_root, "output_root")
        else:
            if self.output_root is not None:
                raise LauncherError("godot mode takes no --output-root (Blender-only)")
            # Godot project binding metadata; a marker, not a full project check.
            _validate_file(
                os.path.join(project_root, "project.godot"), "project.godot"
            )
        object.__setattr__(self, "interpreter", interpreter)
        object.__setattr__(self, "entrypoint", entrypoint)
        object.__setattr__(self, "engine", engine)
        object.__setattr__(self, "project_root", project_root)
        object.__setattr__(self, "runtime_root", runtime_root)
        object.__setattr__(self, "output_root", output_root)

    def runtime_dirs(self) -> dict:
        return {
            name: os.path.join(self.runtime_root, name) for name in RUNTIME_SUBDIRS
        }

    def argv(self) -> list:
        if self.mode == "blender":
            return [self.interpreter, "-B", self.entrypoint]
        return [self.interpreter, self.entrypoint]

    def cwd(self) -> str:
        return self.project_root

    def env(self) -> dict:
        dirs = self.runtime_dirs()
        env = {
            "HOME": dirs["home"],
            "XDG_CONFIG_HOME": dirs["config"],
            "XDG_CACHE_HOME": dirs["cache"],
            "XDG_DATA_HOME": dirs["data"],
            "XDG_STATE_HOME": dirs["state"],
            "XDG_RUNTIME_DIR": dirs["run"],
            "TMPDIR": dirs["tmp"],
            "TMP": dirs["tmp"],
            "TEMP": dirs["tmp"],
            "PATH": os.path.dirname(self.interpreter) + ":/usr/bin:/bin",
            "LANG": "C.UTF-8",
            "LC_ALL": "C.UTF-8",
        }
        if self.mode == "blender":
            env["BLENDER_BIN"] = self.engine
            env["OGS_BLENDER_PROJECT_ROOT"] = self.project_root
            env["OGS_BLENDER_OUTPUT_ROOT"] = self.output_root
            env["PYTHONNOUSERSITE"] = "1"
            env["PYTHONDONTWRITEBYTECODE"] = "1"
        else:
            env["GODOT_PATH"] = self.engine
        return env


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="ogs_mcp_launch",
        description=(
            "Validate explicit absolute launch paths, then execve one managed MCP "
            "runtime. No discovery, config file, provisioning, or engine probe."
        ),
    )
    parser.add_argument("mode", choices=MODES)
    parser.add_argument("--interpreter", required=True)
    parser.add_argument("--entrypoint", required=True)
    parser.add_argument("--engine", required=True)
    parser.add_argument("--project-root", required=True)
    parser.add_argument("--runtime-root", required=True)
    parser.add_argument("--output-root", default=None)
    return parser


def plan_from_argv(argv: list) -> LaunchPlan:
    args = build_parser().parse_args(argv)
    return LaunchPlan(
        mode=args.mode,
        interpreter=args.interpreter,
        entrypoint=args.entrypoint,
        engine=args.engine,
        project_root=args.project_root,
        runtime_root=args.runtime_root,
        output_root=args.output_root,
    )


def exec_plan(plan: LaunchPlan) -> None:
    """Change to the project cwd, then replace this process with the runtime."""
    os.chdir(plan.cwd())
    os.execve(plan.interpreter, plan.argv(), plan.env())


def main(argv: list | None = None) -> int:
    try:
        plan = plan_from_argv(sys.argv[1:] if argv is None else argv)
    except LauncherError as exc:
        print(f"ogs_mcp_launch: {exc}", file=sys.stderr)
        return 2
    try:
        exec_plan(plan)
    except OSError as exc:
        print(
            f"ogs_mcp_launch: cannot exec interpreter {plan.interpreter!r}: {exc}",
            file=sys.stderr,
        )
        return 2
    return 2  # unreachable after a successful execve


if __name__ == "__main__":
    raise SystemExit(main())
