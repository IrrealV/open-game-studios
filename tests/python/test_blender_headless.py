"""Deterministic tests for the Blender headless adapter (stdlib unittest).

No real Blender engine, network, or MCP transport is used. The "engine" is a
small executable Python script written into a per-test temp directory; the MCP
surface is only imported/constructed when the optional ``mcp`` package is
installed (otherwise those tests skip).

Run from the repository root:

    python3 -B -m unittest discover -s tests/python -v
"""

from __future__ import annotations

import asyncio
import importlib.util
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import textwrap
import unittest
import unittest.mock

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
MODULE_PATH = REPO_ROOT / "tools" / "mcp" / "blender_headless.py"


def _load_adapter():
    if not MODULE_PATH.is_file():
        raise RuntimeError(f"adapter module not found: {MODULE_PATH}")
    spec = importlib.util.spec_from_file_location("ogs_blender_headless", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


bh = _load_adapter()

SENTINEL_ENV = {
    "DISPLAY": ":99",
    "DBUS_SESSION_BUS_ADDRESS": "unix:path=/tmp/sentinel-dbus",
    "LD_PRELOAD": "/tmp/sentinel.so",
    "LD_LIBRARY_PATH": "/tmp/sentinel-lib",
    "PYTHONPATH": "/tmp/sentinel-pythonpath",
    "PYTHONHOME": "/tmp/sentinel-pythonhome",
    "HTTPS_PROXY": "http://sentinel-proxy",
    "OPENAI_API_KEY": "sentinel-token",
    "OGS_SENTINEL": "sentinel-harness",
}

DEFAULT_ENGINE = """#!{interpreter}
import json
import os
import sys
import time

if "--version" in sys.argv:
    print("Blender 4.5.3 (fake)")
    raise SystemExit(0)

args = {}
if "--" in sys.argv:
    args = json.loads(sys.argv[sys.argv.index("--") + 1])

mode = args.get("mode", "ok")
payload = {
    "argv": sys.argv,
    "cwd": os.getcwd(),
    "env": dict(os.environ),
    "json_args": args,
}

if mode == "sleep":
    time.sleep(args.get("seconds", 5))
if mode == "big":
    sys.stdout.write("A" * args.get("bytes", 40000))
if mode == "fail":
    sys.stderr.write("engine failure detail\\n")
    print("FAKE " + json.dumps(payload))
    raise SystemExit(args.get("code", 7))

print("FAKE " + json.dumps(payload))
"""

VERSION_FAIL_ENGINE = """#!{interpreter}
import sys
sys.stderr.write("fake version failure\\n")
raise SystemExit(4)
"""

VERSION_SLOW_ENGINE = """#!{interpreter}
import time
time.sleep(30)
"""


class BaseCase(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = pathlib.Path(tmp.name)
        self.project = self.root / "project"
        self.project.mkdir()
        self.output = self.root / "output"
        self.output.mkdir()
        self.engine_dir = self.root / "engine"
        self.engine_dir.mkdir()
        self.blender = self._make_engine(self.engine_dir / "blender")
        self.script = self.project / "build.py"
        self.script.write_text("print('hello')\n")

    def _make_engine(self, path, body=DEFAULT_ENGINE):
        path.write_text(body.replace("{interpreter}", sys.executable))
        path.chmod(0o755)
        return path

    def config(self, **overrides):
        params = {
            "executable": str(self.blender),
            "project_root": str(self.project),
            "output_root": str(self.output),
            "timeout_max_seconds": 5,
        }
        params.update(overrides)
        return bh.HeadlessConfig(**params)

    def parse_payload(self, result):
        line = [ln for ln in result.stdout.splitlines() if ln.startswith("FAKE ")][-1]
        return json.loads(line[len("FAKE "):])

    def sibling_target(self):
        return self.root / "sibling-target"

    def assert_no_writes_in(self, target):
        if target.exists() and target.is_dir() and not target.is_symlink():
            self.assertEqual(list(target.iterdir()), [])
        else:
            self.assertFalse(target.exists())


class ConfigValidationTests(BaseCase):
    def test_from_env_requires_all_three(self):
        with self.assertRaises(bh.BlenderConfigError) as ctx:
            bh.HeadlessConfig.from_env({})
        for name in (bh.ENV_BLENDER_BIN, bh.ENV_PROJECT_ROOT, bh.ENV_OUTPUT_ROOT):
            self.assertIn(name, str(ctx.exception))

    def test_from_env_builds_config(self):
        env = {
            bh.ENV_BLENDER_BIN: str(self.blender),
            bh.ENV_PROJECT_ROOT: str(self.project),
            bh.ENV_OUTPUT_ROOT: str(self.output),
        }
        config = bh.HeadlessConfig.from_env(env)
        self.assertEqual(config.executable, str(self.blender))
        self.assertEqual(config.project_root, str(self.project))
        self.assertEqual(config.output_root, str(self.output))

    def test_relative_executable_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config(executable="blender")

    def test_missing_executable_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config(executable=str(self.root / "missing-blender"))

    def test_nonexecutable_file_rejected(self):
        flat = self.engine_dir / "blender-noexec"
        flat.write_text("#!/bin/sh\nexit 0\n")
        flat.chmod(0o644)
        with self.assertRaises(bh.BlenderConfigError):
            self.config(executable=str(flat))

    def test_symlinked_executable_rejected(self):
        link = self.engine_dir / "blender-link"
        link.symlink_to(self.blender)
        with self.assertRaises(bh.BlenderConfigError):
            self.config(executable=str(link))

    def test_symlinked_root_component_rejected(self):
        link = self.root / "engine-link"
        link.symlink_to(self.engine_dir)
        with self.assertRaises(bh.BlenderConfigError):
            self.config(executable=str(link / "blender"))

    def test_symlinked_project_root_rejected(self):
        link = self.root / "project-link"
        link.symlink_to(self.project)
        with self.assertRaises(bh.BlenderConfigError):
            self.config(project_root=str(link))

    def test_missing_project_root_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config(project_root=str(self.root / "nope"))

    def test_output_root_file_rejected(self):
        flat = self.root / "output-file"
        flat.write_text("x")
        with self.assertRaises(bh.BlenderConfigError):
            self.config(output_root=str(flat))

    def test_timeout_cap_bool_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config(timeout_max_seconds=True)

    def test_timeout_cap_negative_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config(timeout_max_seconds=-1)


class ScriptValidationTests(BaseCase):
    def test_valid_script_accepted(self):
        self.assertEqual(self.config().validate_script(str(self.script)), str(self.script))

    def test_relative_script_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().validate_script("build.py")

    def test_missing_script_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().validate_script(str(self.project / "missing.py"))

    def test_symlinked_script_rejected(self):
        link = self.project / "link.py"
        link.symlink_to(self.script)
        with self.assertRaises(bh.BlenderConfigError):
            self.config().validate_script(str(link))

    def test_symlinked_parent_component_rejected(self):
        outside = self.root / "outside"
        outside.mkdir()
        (outside / "evil.py").write_text("print('evil')\n")
        link = self.project / "linked"
        link.symlink_to(outside)
        with self.assertRaises(bh.BlenderConfigError):
            self.config().validate_script(str(link / "evil.py"))

    def test_script_outside_project_rejected(self):
        outside = self.root / "outside.py"
        outside.write_text("print('out')\n")
        with self.assertRaises(bh.BlenderConfigError):
            self.config().validate_script(str(outside))

    def test_directory_script_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().validate_script(str(self.project))

    def test_no_subprocess_and_no_runtime_on_invalid_script(self):
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.run_script("relative.py")
        run.assert_not_called()
        self.assertFalse((self.output / "runtime").exists())


class TimeoutArgumentTests(BaseCase):
    def test_default_uses_cap(self):
        self.assertEqual(self.config(timeout_max_seconds=9).resolve_timeout(None), 9)

    def test_bool_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().resolve_timeout(True)

    def test_negative_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().resolve_timeout(-1)

    def test_zero_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().resolve_timeout(0)

    def test_float_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().resolve_timeout(1.5)

    def test_too_large_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config(timeout_max_seconds=5).resolve_timeout(6)

    def test_valid_value_accepted(self):
        self.assertEqual(self.config().resolve_timeout(3), 3)

    def test_no_subprocess_on_invalid_timeout(self):
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.run_script(str(self.script), timeout_seconds=True)
        run.assert_not_called()


class ArgsValidationTests(BaseCase):
    def test_non_dict_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().run_script(str(self.script), args=["nope"])

    def test_non_serializable_value_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().run_script(str(self.script), args={"bad": {1, 2}})

    def test_nan_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().run_script(str(self.script), args={"bad": float("nan")})

    def test_integer_key_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().run_script(str(self.script), args={1: "integer-key", "a": "string-key"})

    def test_nested_integer_key_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().run_script(str(self.script), args={"outer": {1: "x"}})

    def test_integer_key_inside_list_rejected(self):
        with self.assertRaises(bh.BlenderConfigError):
            self.config().run_script(str(self.script), args={"items": [{2: "x"}]})

    def test_key_error_message_names_string_requirement(self):
        with self.assertRaises(bh.BlenderConfigError) as ctx:
            self.config().run_script(str(self.script), args={1: "x"})
        self.assertIn("string", str(ctx.exception).lower())

    def test_no_engine_or_runtime_on_key_error(self):
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.run_script(str(self.script), args={1: "x"})
        run.assert_not_called()
        self.assertFalse((self.output / "runtime").exists())

    def test_empty_args_allowed(self):
        self.assertEqual(bh._validate_args(None), {})
        self.assertEqual(bh._validate_args({"k": [1, "two", None]}), {"k": [1, "two", None]})


class RunnerTests(BaseCase):
    def test_success_argv_cwd_and_clean_env(self):
        config = self.config()
        with unittest.mock.patch.dict(os.environ, SENTINEL_ENV):
            result = config.run_script(str(self.script), args={"mode": "ok", "k": "v"})
        self.assertTrue(result.ok, result)
        self.assertEqual(result.exit_code, 0)
        self.assertIsNone(result.error)
        payload = self.parse_payload(result)
        self.assertEqual(
            payload["argv"],
            [
                str(self.blender),
                "--factory-startup",
                "-b",
                "--python-exit-code",
                "1",
                "--python",
                str(self.script),
                "--",
                json.dumps({"mode": "ok", "k": "v"}, sort_keys=True),
            ],
        )
        self.assertEqual(payload["cwd"], str(self.project))
        self.assertEqual(payload["json_args"], {"mode": "ok", "k": "v"})
        child_env = payload["env"]
        for name in SENTINEL_ENV:
            self.assertNotIn(name, child_env, f"{name} leaked into engine env")
        self.assertNotIn("PYTHONPATH", child_env)
        runtime_home = str(self.output / "runtime" / "home")
        self.assertEqual(child_env["HOME"], runtime_home)
        self.assertEqual(child_env["TMPDIR"], str(self.output / "runtime" / "tmp"))
        self.assertEqual(child_env["PYTHONNOUSERSITE"], "1")
        self.assertEqual(child_env["PYTHONDONTWRITEBYTECODE"], "1")
        self.assertEqual(child_env["LC_ALL"], "C.UTF-8")

    def test_nonzero_exit_is_reported_as_error(self):
        result = self.config().run_script(str(self.script), args={"mode": "fail", "code": 7})
        self.assertFalse(result.ok)
        self.assertEqual(result.exit_code, 7)
        self.assertIn("exited 7", result.error or "")
        self.assertIn("engine failure detail", result.stderr)

    def test_timeout_reported(self):
        result = self.config().run_script(
            str(self.script), args={"mode": "sleep", "seconds": 10}, timeout_seconds=1
        )
        self.assertTrue(result.timed_out)
        self.assertFalse(result.ok)
        self.assertIn("timeout", result.error or "")
        self.assertLess(result.duration_seconds, 9)

    def test_oserror_when_engine_disappears(self):
        config = self.config()
        self.blender.unlink()
        result = config.run_script(str(self.script))
        self.assertFalse(result.ok)
        self.assertIsNone(result.exit_code)
        self.assertIn("cannot execute engine", result.error or "")
        self.assertFalse(result.timed_out)

    def test_output_truncation_indicator(self):
        result = self.config().run_script(str(self.script), args={"mode": "big", "bytes": 40000})
        self.assertTrue(result.ok, result)
        self.assertTrue(result.stdout_truncated)
        self.assertEqual(len(result.stdout), bh.MAX_OUTPUT_BYTES)
        self.assertIn("FAKE ", result.stdout)
        self.assertFalse(result.stderr_truncated)

    def test_runtime_dirs_created_after_validation(self):
        config = self.config()
        runtime = self.output / "runtime"
        self.assertFalse(runtime.exists())
        config.run_script(str(self.script))
        self.assertTrue((runtime / "home").is_dir())
        self.assertTrue((runtime / "tmp").is_dir())


class HealthTests(BaseCase):
    def test_health_reports_version(self):
        health = self.config().check_health()
        self.assertTrue(health["ok"])
        self.assertEqual(health["version"], "Blender 4.5.3 (fake)")
        self.assertEqual(health["executable"], str(self.blender))
        self.assertIsNone(health["error"])

    def test_health_nonzero_is_not_ok(self):
        failing = self._make_engine(self.engine_dir / "blender-fail", VERSION_FAIL_ENGINE)
        health = self.config(executable=str(failing)).check_health()
        self.assertFalse(health["ok"])
        self.assertEqual(health["exit_code"], 4)
        self.assertIn("exited 4", health["error"] or "")

    def test_health_timeout(self):
        slow = self._make_engine(self.engine_dir / "blender-slow", VERSION_SLOW_ENGINE)
        health = self.config(executable=str(slow), timeout_max_seconds=1).check_health()
        self.assertTrue(health["timed_out"])
        self.assertFalse(health["ok"])

    def test_health_uses_version_only(self):
        config = self.config()
        completed = subprocess.CompletedProcess(
            args=[], returncode=0, stdout=b"Blender 4.5.3 (fake)\n", stderr=b""
        )
        with unittest.mock.patch.object(bh.subprocess, "run", return_value=completed) as run:
            config.check_health()
        argv = run.call_args.args[0]
        self.assertEqual(argv, [str(self.blender), "--version"])
        self.assertNotIn("--python", argv)


class RuntimePreflightRegressionTests(BaseCase):
    """Defects 1 and 2: symlinked runtime components vs ordinary setup failure."""

    def setUp(self):
        super().setUp()
        self.runtime = self.output / "runtime"
        self.sibling = self.sibling_target()

    def test_runtime_root_symlink_run_script_rejected(self):
        self.sibling.mkdir()
        self.runtime.symlink_to(self.sibling)
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.run_script(str(self.script))
        run.assert_not_called()
        self.assert_no_writes_in(self.sibling)

    def test_runtime_root_symlink_health_rejected(self):
        self.sibling.mkdir()
        self.runtime.symlink_to(self.sibling)
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.check_health()
        run.assert_not_called()
        self.assert_no_writes_in(self.sibling)

    def test_dangling_runtime_symlink_rejected(self):
        self.runtime.symlink_to(self.sibling)  # target intentionally absent
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.run_script(str(self.script))
        run.assert_not_called()
        self.assert_no_writes_in(self.sibling)

    def test_nested_home_symlink_rejected_before_other_writes(self):
        self.runtime.mkdir()
        self.sibling.mkdir()
        (self.runtime / "home").symlink_to(self.sibling)
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                config.run_script(str(self.script))
        run.assert_not_called()
        self.assert_no_writes_in(self.sibling)
        self.assertFalse((self.runtime / "tmp").exists())

    def test_nested_tmp_symlink_rejected(self):
        self.runtime.mkdir()
        self.sibling.mkdir()
        (self.runtime / "tmp").symlink_to(self.sibling)
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                self.config().run_script(str(self.script))
        run.assert_not_called()
        self.assert_no_writes_in(self.sibling)

    def test_nested_xdg_cache_symlink_rejected(self):
        (self.runtime / "home").mkdir(parents=True)
        self.sibling.mkdir()
        (self.runtime / "home" / ".cache").symlink_to(self.sibling)
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with self.assertRaises(bh.BlenderConfigError):
                self.config().run_script(str(self.script))
        run.assert_not_called()
        self.assert_no_writes_in(self.sibling)

    def test_ordinary_file_at_runtime_is_structured_failure(self):
        self.runtime.write_text("not a directory")
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            result = config.run_script(str(self.script))
        run.assert_not_called()
        self.assertFalse(result.ok)
        self.assertIn("runtime", result.error or "")

    def test_ordinary_file_at_runtime_health_is_structured_failure(self):
        self.runtime.write_text("not a directory")
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            health = config.check_health()
        run.assert_not_called()
        self.assertFalse(health["ok"])
        self.assertIn("runtime", health["error"] or "")

    def test_ordinary_file_at_home_is_structured_failure(self):
        self.runtime.mkdir()
        (self.runtime / "home").write_text("not a directory")
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            result = self.config().run_script(str(self.script))
        run.assert_not_called()
        self.assertFalse(result.ok)
        self.assertIn("runtime", result.error or "")

    def test_preparation_permission_error_is_structured(self):
        config = self.config()
        with unittest.mock.patch.object(bh.subprocess, "run") as run:
            with unittest.mock.patch.object(
                bh.os, "makedirs", side_effect=PermissionError("denied")
            ):
                result = config.run_script(str(self.script))
        run.assert_not_called()
        self.assertFalse(result.ok)
        self.assertIn("denied", result.error or "")


def _mcp_available():
    try:
        importlib.import_module("mcp.server.fastmcp")
    except ImportError:
        return False
    return True


@unittest.skipUnless(_mcp_available(), "optional 'mcp' package is not installed")
class ToolSurfaceTests(BaseCase):
    def test_exactly_two_tools(self):
        server = bh.create_server(self.config())
        tools = asyncio.run(server.list_tools())
        self.assertEqual(sorted(t.name for t in tools), ["blender_python_exec", "health"])

    def test_tool_descriptions_disclose_trust_model(self):
        server = bh.create_server(self.config())
        tools = {t.name: t for t in asyncio.run(server.list_tools())}
        self.assertIn("not a sandbox", tools["blender_python_exec"].description.lower())
        self.assertTrue(tools["health"].description)


BLOCK_MCP_SCRIPT = textwrap.dedent(
    """
    import importlib.abc
    import importlib.util
    import sys

    class Blocker(importlib.abc.MetaPathFinder):
        def find_spec(self, fullname, path=None, target=None):
            if fullname == "mcp" or fullname.startswith("mcp."):
                raise ImportError("mcp blocked for test")
            return None

    sys.meta_path.insert(0, Blocker())
    spec = importlib.util.spec_from_file_location("ogs_blender_headless", sys.argv[1])
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    try:
        import mcp  # noqa: F401
    except ImportError:
        pass
    else:
        raise SystemExit("mcp was importable despite the blocker")
    assert callable(module.main)
    assert callable(module.HeadlessConfig.from_env)
    print("LAZY-IMPORT-OK")
    """
)


class LazyImportTests(unittest.TestCase):
    def test_module_imports_without_mcp(self):
        completed = subprocess.run(
            [sys.executable, "-B", "-c", BLOCK_MCP_SCRIPT, str(MODULE_PATH)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertIn("LAZY-IMPORT-OK", completed.stdout)


if __name__ == "__main__":
    unittest.main()
