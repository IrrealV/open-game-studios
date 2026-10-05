"""Deterministic tests for the OGS MCP persistent launcher (stdlib unittest).

The launcher never starts a real engine, node runtime, or MCP server in these
tests: most cases mock ``os.execve``; the cwd-binding cases execute only forged
interpreters inside per-test temporary directories. This keeps the suite
offline and makes the exact argv/environment contract assertable.

Run from the repository root:

    python3 -B -m unittest discover -s tests/python -v
"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest
import unittest.mock

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
MODULE_PATH = REPO_ROOT / "tools" / "mcp" / "ogs_mcp_launch.py"


def _load_launcher():
    if not MODULE_PATH.is_file():
        raise RuntimeError(f"launcher module not found: {MODULE_PATH}")
    spec = importlib.util.spec_from_file_location("ogs_mcp_launch", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


launch = _load_launcher()

BASE_ENV_KEYS = {
    "HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME",
    "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP",
    "PATH", "LANG", "LC_ALL",
}
BLENDER_ENV_KEYS = BASE_ENV_KEYS | {
    "BLENDER_BIN", "OGS_BLENDER_PROJECT_ROOT", "OGS_BLENDER_OUTPUT_ROOT",
    "PYTHONNOUSERSITE", "PYTHONDONTWRITEBYTECODE",
}
GODOT_ENV_KEYS = BASE_ENV_KEYS | {"GODOT_PATH"}

SENTINEL_ENV = {
    "DISPLAY": ":99",
    "LD_PRELOAD": "/tmp/sentinel.so",
    "LD_LIBRARY_PATH": "/tmp/sentinel-lib",
    "PYTHONPATH": "/tmp/sentinel-pythonpath",
    "NODE_OPTIONS": "--require /tmp/sentinel.js",
    "HTTPS_PROXY": "http://sentinel-proxy",
    "OPENAI_API_KEY": "sentinel-token",
    "OGS_SENTINEL": "sentinel-harness",
}


class BaseCase(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = pathlib.Path(tmp.name)

        self.project = self.root / "project"
        self.project.mkdir()
        (self.project / "project.godot").write_text("[application]\n")

        self.runtime = self.root / "runtime"
        self._make_runtime(self.runtime)

        self.output = self.root / "output"
        self.output.mkdir()

        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.interpreter = self._make_executable(self.bin / "python-3.14")
        self.godot_interpreter = self._make_executable(self.bin / "node")

        self.engine_dir = self.root / "engines"
        self.engine_dir.mkdir()
        self.blender = self._make_executable(self.engine_dir / "blender")
        self.godot = self._make_executable(self.engine_dir / "godot")

        self.server_dir = self.root / "server"
        self.server_dir.mkdir()
        self.entrypoint = self.server_dir / "entrypoint.py"
        self.entrypoint.write_text("# blender mcp entrypoint\n")
        self.godot_entrypoint = self.server_dir / "index.js"
        self.godot_entrypoint.write_text("// prebuilt godot mcp index\n")

    def _make_executable(self, path):
        path.write_text("#!/bin/sh\nexit 0\n")
        path.chmod(0o755)
        return path

    def _make_fake_interpreter(self, path, report):
        """A real executable that records its cwd/argv/env, then exits."""
        body = (
            "#!" + sys.executable + "\n"
            "import json\n"
            "import os\n"
            "import sys\n\n"
            f"REPORT = {str(report)!r}\n"
            "with open(REPORT, 'w') as handle:\n"
            "    json.dump({'cwd': os.getcwd(), 'argv': sys.argv,\n"
            "               'env': dict(os.environ)}, handle, sort_keys=True)\n"
        )
        path.write_text(body)
        path.chmod(0o755)
        return path

    def _make_runtime(self, root):
        root.mkdir()
        for name in launch.RUNTIME_SUBDIRS:
            (root / name).mkdir()

    def plan(self, **overrides):
        params = {
            "mode": "blender",
            "interpreter": str(self.interpreter),
            "entrypoint": str(self.entrypoint),
            "engine": str(self.blender),
            "project_root": str(self.project),
            "runtime_root": str(self.runtime),
            "output_root": str(self.output),
        }
        params.update(overrides)
        return launch.LaunchPlan(**params)

    def blender_argv(self, **overrides):
        plan = self.plan(**overrides)
        return plan.argv()

    def snapshot(self, root):
        entries = {}
        for path in sorted(root.rglob("*")):
            rel = str(path.relative_to(root))
            if path.is_symlink():
                entries[rel] = ("symlink", os.readlink(path))
            elif path.is_dir():
                entries[rel] = ("dir", None)
            else:
                entries[rel] = ("file", path.read_bytes())
        return entries

    def assert_rejected(self, **overrides):
        before = self.snapshot(self.root)
        with unittest.mock.patch.object(launch.os, "execve") as execve:
            with self.assertRaises(launch.LauncherError):
                self.plan(**overrides)
        execve.assert_not_called()
        self.assertEqual(self.snapshot(self.root), before)
        return overrides


class PlanValidationTests(BaseCase):
    def test_valid_blender_plan_builds(self):
        plan = self.plan()
        self.assertEqual(plan.mode, "blender")
        self.assertEqual(plan.interpreter, str(self.interpreter))
        self.assertEqual(plan.output_root, str(self.output))

    def test_valid_godot_plan_builds(self):
        plan = self.plan(
            mode="godot", interpreter=str(self.godot_interpreter),
            entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
            output_root=None,
        )
        self.assertIsNone(plan.output_root)

    def test_relative_interpreter_rejected(self):
        self.assert_rejected(interpreter="bin/python")

    def test_missing_interpreter_rejected(self):
        self.assert_rejected(interpreter=str(self.root / "missing-python"))

    def test_nonexecutable_interpreter_rejected(self):
        flat = self.bin / "python-flat"
        flat.write_text("#!/bin/sh\nexit 0\n")
        flat.chmod(0o644)
        self.assert_rejected(interpreter=str(flat))

    def test_symlinked_interpreter_rejected(self):
        link = self.bin / "python-link"
        link.symlink_to(self.interpreter)
        self.assert_rejected(interpreter=str(link))

    def test_interpreter_symlinked_component_rejected(self):
        link = self.root / "bin-link"
        link.symlink_to(self.bin)
        self.assert_rejected(interpreter=str(link / "python-3.14"))

    def test_missing_engine_rejected(self):
        self.assert_rejected(engine=str(self.root / "missing-blender"))

    def test_nonexecutable_engine_rejected(self):
        flat = self.engine_dir / "blender-flat"
        flat.write_text("x")
        flat.chmod(0o644)
        self.assert_rejected(engine=str(flat))

    def test_relative_engine_rejected(self):
        self.assert_rejected(engine="blender")

    def test_missing_entrypoint_rejected(self):
        self.assert_rejected(entrypoint=str(self.server_dir / "missing.py"))

    def test_directory_entrypoint_rejected(self):
        self.assert_rejected(entrypoint=str(self.server_dir))

    def test_symlinked_entrypoint_rejected(self):
        link = self.server_dir / "entrypoint-link.py"
        link.symlink_to(self.entrypoint)
        self.assert_rejected(entrypoint=str(link))

    def test_missing_project_root_rejected(self):
        self.assert_rejected(project_root=str(self.root / "missing-project"))

    def test_file_project_root_rejected(self):
        flat = self.root / "project-file"
        flat.write_text("x")
        self.assert_rejected(project_root=str(flat))

    def test_missing_runtime_root_rejected(self):
        self.assert_rejected(runtime_root=str(self.root / "missing-runtime"))

    def test_missing_runtime_subdir_rejected(self):
        (self.runtime / "tmp").rmdir()
        self.assert_rejected()

    def test_symlinked_runtime_subdir_rejected(self):
        target = self.root / "runtime-target"
        target.mkdir()
        (self.runtime / "data").rmdir()
        (self.runtime / "data").symlink_to(target)
        self.assert_rejected()

    def test_blender_requires_output_root(self):
        self.assert_rejected(output_root=None)

    def test_godot_rejects_output_root(self):
        with self.assertRaises(launch.LauncherError):
            self.plan(
                mode="godot", interpreter=str(self.godot_interpreter),
                entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
            )
        # Godot without --output-root is valid.
        plan = self.plan(
            mode="godot", interpreter=str(self.godot_interpreter),
            entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
            output_root=None,
        )
        self.assertEqual(plan.mode, "godot")

    def test_godot_requires_project_marker(self):
        (self.project / "project.godot").unlink()
        with self.assertRaises(launch.LauncherError) as ctx:
            self.plan(
                mode="godot", interpreter=str(self.godot_interpreter),
                entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
                output_root=None,
            )
        self.assertIn("project.godot", str(ctx.exception))

    def test_blender_does_not_require_project_marker(self):
        (self.project / "project.godot").unlink()
        self.assertEqual(self.plan().mode, "blender")

    def test_tmp_fixture_paths_are_accepted(self):
        # A generic launcher must not blanket-reject /tmp: legitimate test and
        # ephemeral runtime fixtures live there. Production refusal is policy.
        self.assertTrue(str(self.root).startswith("/tmp/"))
        self.assertEqual(self.plan().runtime_root, str(self.runtime))


class ParserTests(BaseCase):
    def test_unknown_mode_exits_2(self):
        err = io.StringIO()
        with self.assertRaises(SystemExit) as ctx, contextlib.redirect_stderr(err):
            launch.plan_from_argv(["unity", "--interpreter", str(self.interpreter)])
        self.assertEqual(ctx.exception.code, 2)
        self.assertIn("invalid choice", err.getvalue())

    def test_missing_required_flag_exits_2(self):
        with self.assertRaises(SystemExit) as ctx, contextlib.redirect_stderr(io.StringIO()):
            launch.plan_from_argv(
                ["blender", "--entrypoint", str(self.entrypoint),
                 "--engine", str(self.blender), "--project-root", str(self.project),
                 "--runtime-root", str(self.runtime), "--output-root", str(self.output)]
            )
        self.assertEqual(ctx.exception.code, 2)

    def test_parser_builds_plan(self):
        plan = launch.plan_from_argv([
            "blender", "--interpreter", str(self.interpreter),
            "--entrypoint", str(self.entrypoint), "--engine", str(self.blender),
            "--project-root", str(self.project), "--runtime-root", str(self.runtime),
            "--output-root", str(self.output),
        ])
        self.assertEqual(plan.argv(), [str(self.interpreter), "-B", str(self.entrypoint)])


class LaunchContractTests(BaseCase):
    def test_blender_argv_is_exact(self):
        self.assertEqual(
            self.blender_argv(),
            [str(self.interpreter), "-B", str(self.entrypoint)],
        )

    def test_godot_argv_is_exact(self):
        plan = self.plan(
            mode="godot", interpreter=str(self.godot_interpreter),
            entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
            output_root=None,
        )
        self.assertEqual(plan.argv(), [str(self.godot_interpreter), str(self.godot_entrypoint)])

    def test_cwd_is_project_root_in_both_modes(self):
        self.assertEqual(self.plan().cwd(), str(self.project))
        godot = self.plan(
            mode="godot", interpreter=str(self.godot_interpreter),
            entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
            output_root=None,
        )
        self.assertEqual(godot.cwd(), str(self.project))

    def test_blender_env_is_exact_allowlist(self):
        env = self.plan().env()
        self.assertEqual(set(env), BLENDER_ENV_KEYS)
        self.assertEqual(env["BLENDER_BIN"], str(self.blender))
        self.assertEqual(env["OGS_BLENDER_PROJECT_ROOT"], str(self.project))
        self.assertEqual(env["OGS_BLENDER_OUTPUT_ROOT"], str(self.output))
        self.assertEqual(env["PYTHONNOUSERSITE"], "1")
        self.assertEqual(env["PYTHONDONTWRITEBYTECODE"], "1")

    def test_godot_env_is_exact_allowlist(self):
        plan = self.plan(
            mode="godot", interpreter=str(self.godot_interpreter),
            entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
            output_root=None,
        )
        env = plan.env()
        self.assertEqual(set(env), GODOT_ENV_KEYS)
        self.assertEqual(env["GODOT_PATH"], str(self.godot))
        self.assertNotIn("BLENDER_BIN", env)

    def test_runtime_env_points_inside_runtime_root(self):
        env = self.plan().env()
        self.assertEqual(env["HOME"], str(self.runtime / "home"))
        self.assertEqual(env["TMPDIR"], str(self.runtime / "tmp"))
        self.assertEqual(env["XDG_CONFIG_HOME"], str(self.runtime / "config"))
        self.assertEqual(env["XDG_CACHE_HOME"], str(self.runtime / "cache"))
        self.assertEqual(env["XDG_DATA_HOME"], str(self.runtime / "data"))
        self.assertEqual(env["XDG_STATE_HOME"], str(self.runtime / "state"))
        self.assertEqual(env["XDG_RUNTIME_DIR"], str(self.runtime / "run"))
        self.assertEqual(env["PATH"], str(self.bin) + ":/usr/bin:/bin")
        self.assertEqual(env["LANG"], "C.UTF-8")
        self.assertEqual(env["LC_ALL"], "C.UTF-8")

    def test_env_never_inherits_process_environment(self):
        with unittest.mock.patch.dict(os.environ, SENTINEL_ENV, clear=False):
            blender_env = self.plan().env()
            godot = self.plan(
                mode="godot", interpreter=str(self.godot_interpreter),
                entrypoint=str(self.godot_entrypoint), engine=str(self.godot),
                output_root=None,
            )
            godot_env = godot.env()
        for name in SENTINEL_ENV:
            self.assertNotIn(name, blender_env)
            self.assertNotIn(name, godot_env)


class ExecutionTests(BaseCase):
    def test_exec_plan_calls_execve_once(self):
        plan = self.plan()
        with unittest.mock.patch.object(launch.os, "chdir"), \
             unittest.mock.patch.object(launch.os, "execve") as execve:
            launch.exec_plan(plan)
        execve.assert_called_once_with(str(self.interpreter), plan.argv(), plan.env())

    def test_exec_plan_chdirs_to_project_cwd_before_execve(self):
        plan = self.plan()
        calls = []
        with unittest.mock.patch.object(
            launch.os, "chdir", side_effect=lambda path: calls.append(("chdir", path))
        ), unittest.mock.patch.object(
            launch.os, "execve", side_effect=lambda *a: calls.append(("execve", a))
        ):
            launch.exec_plan(plan)
        self.assertEqual(calls, [
            ("chdir", str(self.project)),
            ("execve", (str(self.interpreter), plan.argv(), plan.env())),
        ])

    def test_exec_plan_chdir_oserror_prevents_exec(self):
        with unittest.mock.patch.object(
            launch.os, "chdir", side_effect=OSError("chdir denied")
        ), unittest.mock.patch.object(launch.os, "execve") as execve:
            with self.assertRaises(OSError):
                launch.exec_plan(self.plan())
        execve.assert_not_called()

    def test_main_chdir_oserror_returns_2_with_stderr_and_silent_stdout(self):
        out, err = io.StringIO(), io.StringIO()
        with unittest.mock.patch.object(
            launch.os, "chdir", side_effect=OSError("chdir denied")
        ), unittest.mock.patch.object(launch.os, "execve") as execve, \
             contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = launch.main([
                "blender", "--interpreter", str(self.interpreter),
                "--entrypoint", str(self.entrypoint), "--engine", str(self.blender),
                "--project-root", str(self.project), "--runtime-root", str(self.runtime),
                "--output-root", str(self.output),
            ])
        self.assertEqual(code, 2)
        execve.assert_not_called()
        self.assertEqual(out.getvalue(), "")
        self.assertIn("chdir denied", err.getvalue())
        self.assertIn(str(self.interpreter), err.getvalue())

    def test_main_execs_with_exact_plan_and_silent_stdout(self):
        out, err = io.StringIO(), io.StringIO()
        with unittest.mock.patch.object(launch.os, "chdir"), \
             unittest.mock.patch.object(launch.os, "execve") as execve:
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                launch.main([
                    "blender", "--interpreter", str(self.interpreter),
                    "--entrypoint", str(self.entrypoint), "--engine", str(self.blender),
                    "--project-root", str(self.project), "--runtime-root", str(self.runtime),
                    "--output-root", str(self.output),
                ])
        self.assertEqual(execve.call_count, 1)
        path, argv, env = execve.call_args.args
        self.assertEqual(path, str(self.interpreter))
        self.assertEqual(argv, [str(self.interpreter), "-B", str(self.entrypoint)])
        self.assertEqual(set(env), BLENDER_ENV_KEYS)
        self.assertEqual(out.getvalue(), "")
        self.assertEqual(err.getvalue(), "")

    def test_main_invalid_path_returns_2_with_actionable_stderr(self):
        before = self.snapshot(self.root)
        err = io.StringIO()
        with unittest.mock.patch.object(launch.os, "execve") as execve:
            with contextlib.redirect_stderr(err):
                code = launch.main([
                    "blender", "--interpreter", "relative-python",
                    "--entrypoint", str(self.entrypoint), "--engine", str(self.blender),
                    "--project-root", str(self.project), "--runtime-root", str(self.runtime),
                    "--output-root", str(self.output),
                ])
        self.assertEqual(code, 2)
        execve.assert_not_called()
        self.assertIn("interpreter", err.getvalue())
        self.assertEqual(self.snapshot(self.root), before)

    def test_main_exec_oserror_returns_2_with_actionable_stderr(self):
        err = io.StringIO()
        with unittest.mock.patch.object(launch.os, "chdir"), \
             unittest.mock.patch.object(
                 launch.os, "execve", side_effect=OSError("Exec format error")
             ):
            with contextlib.redirect_stderr(err):
                code = launch.main([
                    "blender", "--interpreter", str(self.interpreter),
                    "--entrypoint", str(self.entrypoint), "--engine", str(self.blender),
                    "--project-root", str(self.project), "--runtime-root", str(self.runtime),
                    "--output-root", str(self.output),
                ])
        self.assertEqual(code, 2)
        self.assertIn(str(self.interpreter), err.getvalue())
        self.assertIn("Exec format error", err.getvalue())

    def test_validation_leaves_preprovisioned_bytes_unchanged(self):
        marker = self.runtime / "home" / "sentinel.txt"
        marker.write_bytes(b"preprovisioned\n")
        before = self.snapshot(self.root)
        with unittest.mock.patch.object(launch.os, "execve"):
            with self.assertRaises(launch.LauncherError):
                self.plan(entrypoint=str(self.server_dir / "missing.py"))
        self.assertEqual(marker.read_bytes(), b"preprovisioned\n")
        self.assertEqual(self.snapshot(self.root), before)


class RealExecBindingTests(BaseCase):
    """A real forged-interpreter subprocess: exact cwd and mode env binding."""

    HELPER = (
        "import importlib.util, sys\n"
        "spec = importlib.util.spec_from_file_location('ogs_mcp_launch', sys.argv[1])\n"
        "module = importlib.util.module_from_spec(spec)\n"
        "sys.modules[spec.name] = module\n"
        "spec.loader.exec_module(module)\n"
        "module.main(sys.argv[2:])\n"
    )

    def test_real_subprocess_rebinds_cwd_and_binds_mode_env(self):
        caller = self.root / "caller"
        caller.mkdir()
        for mode, engine, marker, other in (
            ("blender", self.blender, "BLENDER_BIN", "GODOT_PATH"),
            ("godot", self.godot, "GODOT_PATH", "BLENDER_BIN"),
        ):
            with self.subTest(mode=mode):
                report = self.root / f"report-{mode}.json"
                interpreter = self._make_fake_interpreter(
                    self.bin / f"fake-{mode}", report
                )
                args = [
                    mode, "--interpreter", str(interpreter),
                    "--entrypoint", str(self.entrypoint), "--engine", str(engine),
                    "--project-root", str(self.project),
                    "--runtime-root", str(self.runtime),
                ]
                if mode == "blender":
                    args += ["--output-root", str(self.output)]
                completed = subprocess.run(
                    [sys.executable, "-B", "-c", self.HELPER, str(MODULE_PATH)] + args,
                    cwd=str(caller), capture_output=True, text=True, check=False,
                )
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual(completed.stdout, "")
                data = json.loads(report.read_text())
                self.assertEqual(data["cwd"], str(self.project))
                self.assertNotEqual(data["cwd"], str(caller))
                self.assertIn(marker, data["env"])
                self.assertNotIn(other, data["env"])
                expected = (
                    [str(interpreter), "-B", str(self.entrypoint)]
                    if mode == "blender"
                    else [str(interpreter), str(self.entrypoint)]
                )
                self.assertEqual(data["argv"], expected)


if __name__ == "__main__":
    unittest.main()
