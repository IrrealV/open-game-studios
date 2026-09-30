import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { access, cp, mkdir, mkdtemp, readFile, readdir, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const fixtureRoot = path.join(repoRoot, "testdata", "godot-minimal-2d");
const fixtureFiles = ["project.godot", "player.tscn", "player.gd", "verify_mechanic.gd"];
const fixtureSnapshot = new Map(
  await Promise.all(fixtureFiles.map(async (name) => [name, await readFile(path.join(fixtureRoot, name), "utf8")])),
);
const godotBin = process.env.GODOT_BIN?.trim() ?? "";
const runtimeOptions = godotBin === ""
  ? { skip: "GODOT_BIN is absent; structural-only run intentionally skips real Godot" }
  : {};

async function assertMissing(target) {
  await assert.rejects(access(target), (error) => error?.code === "ENOENT");
}

async function createIsolatedProject() {
  const prefix = path.join(tmpdir(), "ogs-godot-change-");
  const root = await mkdtemp(prefix);
  assert.ok(root.startsWith(prefix));

  const project = path.join(root, "project");
  const sandbox = path.join(root, "sandbox");
  await cp(fixtureRoot, project, { recursive: true });
  for (const directory of ["home", "config", "data", "cache", "runtime", "tmp"]) {
    await mkdir(path.join(sandbox, directory), { recursive: true, mode: 0o700 });
  }
  return { root, project, sandbox };
}

function runGodot(isolation, expectedSpeed) {
  return new Promise((resolve, reject) => {
    const args = [
      "--headless",
      "--path",
      isolation.project,
      "--script",
      "res://verify_mechanic.gd",
      "--",
      `--expected-speed=${expectedSpeed}`,
    ];
    const env = {
      ...process.env,
      HOME: path.join(isolation.sandbox, "home"),
      XDG_CONFIG_HOME: path.join(isolation.sandbox, "config"),
      XDG_DATA_HOME: path.join(isolation.sandbox, "data"),
      XDG_CACHE_HOME: path.join(isolation.sandbox, "cache"),
      XDG_RUNTIME_DIR: path.join(isolation.sandbox, "runtime"),
      TMPDIR: path.join(isolation.sandbox, "tmp"),
    };
    const child = spawn(godotBin, args, { cwd: isolation.project, env, stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGKILL");
    }, 20_000);

    child.stdout.setEncoding("utf8").on("data", (chunk) => { stdout += chunk; });
    child.stderr.setEncoding("utf8").on("data", (chunk) => { stderr += chunk; });
    child.once("error", (error) => {
      clearTimeout(timer);
      reject(error);
    });
    child.once("close", (code, signal) => {
      clearTimeout(timer);
      resolve({ code, signal, stdout, stderr, timedOut });
    });
  });
}

function measurementFrom(result) {
  const line = `${result.stdout}\n${result.stderr}`
    .split(/\r?\n/u)
    .find((candidate) => candidate.startsWith("OGS_MEASUREMENT "));
  assert.ok(line, `missing measurement output\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
  return JSON.parse(line.slice("OGS_MEASUREMENT ".length));
}

function assertCompleted(result, expectedCode) {
  assert.equal(result.timedOut, false, "Godot exceeded the 20 second timeout");
  assert.equal(result.signal, null, `Godot ended from signal ${result.signal}`);
  assert.equal(result.code, expectedCode, `unexpected Godot exit\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
}

function extractDestinationPreflight(setup) {
  const fragment = setup.match(/# DESTINATION PREFLIGHT BEGIN\n([\s\S]*?)\n# DESTINATION PREFLIGHT END/u)?.[1];
  assert.ok(fragment, "delimited destination preflight is missing");
  return fragment;
}

async function createGuardSandbox() {
  const prefix = path.join(tmpdir(), "ogs-godot-change-");
  const root = await mkdtemp(prefix);
  assert.ok(root.startsWith(prefix));
  const home = path.join(root, "home");
  await mkdir(home, { mode: 0o700 });
  return { root, home };
}

function runDestinationPreflight(fragment, sandbox) {
  return new Promise((resolve, reject) => {
    const child = spawn("/bin/sh", ["-eu", "-c", fragment], {
      cwd: sandbox.root,
      env: { ...process.env, HOME: sandbox.home, TMPDIR: sandbox.root },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGKILL");
    }, 5_000);

    child.stdout.setEncoding("utf8").on("data", (chunk) => { stdout += chunk; });
    child.stderr.setEncoding("utf8").on("data", (chunk) => { stderr += chunk; });
    child.once("error", (error) => {
      clearTimeout(timer);
      reject(error);
    });
    child.once("close", (code, signal) => {
      clearTimeout(timer);
      resolve({ code, signal, stdout, stderr, timedOut });
    });
  });
}

function assertGuardCompleted(result, expectedCode) {
  assert.equal(result.timedOut, false, "destination preflight exceeded the 5 second timeout");
  assert.equal(result.signal, null, `destination preflight ended from signal ${result.signal}`);
  assert.equal(result.code, expectedCode, `unexpected preflight exit\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
}

test("package manifest exposes the approved skills and has no dependency surface", async () => {
  const manifest = JSON.parse(await readFile(path.join(repoRoot, "package.json"), "utf8"));
  assert.equal(manifest.private, true);
  assert.deepEqual(manifest.pi, { skills: ["./skills/ogs-godot-change", "./skills/ogs-core"] });
  for (const field of ["scripts", "dependencies", "devDependencies", "peerDependencies", "optionalDependencies", "bundledDependencies"]) {
    assert.equal(Object.hasOwn(manifest, field), false, `${field} must remain absent`);
  }
  for (const lockfile of ["package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock"]) {
    await assertMissing(path.join(repoRoot, lockfile));
  }
});

test("skill and local reference expose bounded runtime guidance", async () => {
  const skill = await readFile(path.join(repoRoot, "skills", "ogs-godot-change", "SKILL.md"), "utf8");
  const setup = await readFile(path.join(repoRoot, "skills", "ogs-godot-change", "references", "godot-setup.md"), "utf8");
  assert.match(skill, /^---\nname: ogs-godot-change\n/mu);
  assert.match(skill, /\(references\/godot-setup\.md\)/u);
  for (const heading of ["Activation Contract", "Hard Rules", "Decision Gates", "Execution Steps", "Output Contract", "References"]) {
    assert.match(skill, new RegExp(`^## ${heading}$`, "mu"));
  }
  assert.match(setup, /STOP and await explicit approval/u);
  assert.match(setup, /77860424/u);
  assert.match(setup, /cadd3204e728a35d3f13adb7fd0d7902636b79f6b95c40c265eb73b6c35329e4/u);
  assert.match(setup, /Godot_v4\.7\.2-stable_linux\.x86_64/u);
  assert.match(setup, /not a deterministic installer or consent-enforcement tool/u);
  assert.match(setup, /development installation used Python `urllib`, `hashlib`, and `zipfile`/u);
  assert.match(setup, /has not been downloaded or executed end-to-end/u);
  const commandBlock = setup.match(/Exact proposed installation commands:\n\n```sh\n([\s\S]*?)\n```/u)?.[1];
  assert.ok(commandBlock, "installation command block is missing");
  assert.doesNotMatch(commandBlock, /curl[^\n]*\|[^\n]*(?:sh|bash)/u);
  assert.doesNotMatch(extractDestinationPreflight(setup), /\b(?:curl|wget|unzip|mkdir)\b/u);
});

test("destination preflight guard accepts a fresh path without creating it", async () => {
  const sandbox = await createGuardSandbox();
  const regularParent = path.join(sandbox.home, ".local", "share", "ogs", "tools", "godot");
  const destinationDirectory = path.join(regularParent, "4.7.2");
  await mkdir(regularParent, { recursive: true, mode: 0o700 });
  const setup = await readFile(path.join(repoRoot, "skills", "ogs-godot-change", "references", "godot-setup.md"), "utf8");

  const result = await runDestinationPreflight(extractDestinationPreflight(setup), sandbox);

  assertGuardCompleted(result, 0);
  await assertMissing(destinationDirectory);
});

test("destination preflight guard rejects a preexisting final destination", async () => {
  const sandbox = await createGuardSandbox();
  const destinationDirectory = path.join(sandbox.home, ".local", "share", "ogs", "tools", "godot", "4.7.2");
  const destination = path.join(destinationDirectory, "Godot_v4.7.2-stable_linux.x86_64");
  await mkdir(destinationDirectory, { recursive: true, mode: 0o700 });
  await writeFile(destination, "preserve", { mode: 0o600 });
  const setup = await readFile(path.join(repoRoot, "skills", "ogs-godot-change", "references", "godot-setup.md"), "utf8");

  const result = await runDestinationPreflight(extractDestinationPreflight(setup), sandbox);

  assertGuardCompleted(result, 1);
  assert.match(result.stderr, /final destination already exists/u);
  assert.equal(await readFile(destination, "utf8"), "preserve");
});

test("destination preflight guard rejects a symlink ancestor without target writes", async () => {
  const sandbox = await createGuardSandbox();
  const externalTarget = path.join(sandbox.root, "external-target");
  const linkParent = path.join(sandbox.home, ".local", "share");
  await mkdir(externalTarget, { mode: 0o700 });
  await mkdir(linkParent, { recursive: true, mode: 0o700 });
  await symlink(externalTarget, path.join(linkParent, "ogs"), "dir");
  const setup = await readFile(path.join(repoRoot, "skills", "ogs-godot-change", "references", "godot-setup.md"), "utf8");

  const result = await runDestinationPreflight(extractDestinationPreflight(setup), sandbox);

  assertGuardCompleted(result, 1);
  assert.match(result.stderr, /symlink destination ancestor rejected/u);
  assert.deepEqual(await readdir(externalTarget), []);
});

test("fixture defines visible geometry and a real movement controller", async () => {
  const scene = fixtureSnapshot.get("player.tscn");
  const controller = fixtureSnapshot.get("player.gd");
  const verifier = fixtureSnapshot.get("verify_mechanic.gd");
  assert.match(scene, /Polygon2D/u);
  assert.match(scene, /CollisionShape2D/u);
  assert.match(controller, /@export var speed: float = 120\.0/u);
  assert.match(controller, /move_and_collide\(velocity \* delta\)/u);
  assert.match(verifier, /player\.physics_step\(Vector2\.RIGHT, FIXED_DELTA\)/u);
  assert.match(verifier, /OGS_MEASUREMENT/u);
  await assertMissing(path.join(fixtureRoot, ".godot"));
});

test("Godot measures baseline controller displacement", runtimeOptions, async () => {
  const isolation = await createIsolatedProject();
  const result = await runGodot(isolation, "120");
  assertCompleted(result, 0);
  const measurement = measurementFrom(result);
  assert.equal(measurement.status, "pass");
  assert.equal(measurement.measured_distance, 24);
  console.log(`Godot baseline measurement: ${JSON.stringify(measurement)}`);
});

test("Godot measures changed speed on a temporary project copy", runtimeOptions, async () => {
  const isolation = await createIsolatedProject();
  const controllerPath = path.join(isolation.project, "player.gd");
  const original = await readFile(controllerPath, "utf8");
  const changed = original.replace("@export var speed: float = 120.0", "@export var speed: float = 240.0");
  assert.notEqual(changed, original, "temporary speed edit did not apply");
  await writeFile(controllerPath, changed, "utf8");

  const result = await runGodot(isolation, "240");
  assertCompleted(result, 0);
  const measurement = measurementFrom(result);
  assert.equal(measurement.status, "pass");
  assert.equal(measurement.measured_distance, 48);
  console.log(`Godot changed-speed measurement: ${JSON.stringify(measurement)}`);
});

test("Godot returns nonzero for a failed movement expectation", runtimeOptions, async () => {
  const isolation = await createIsolatedProject();
  const result = await runGodot(isolation, "240");
  assertCompleted(result, 1);
  const measurement = measurementFrom(result);
  assert.equal(measurement.status, "fail");
  assert.equal(measurement.measured_distance, 24);
  console.log(`Godot failed-expectation measurement: ${JSON.stringify(measurement)}`);
});

test("Godot returns nonzero for invalid expected-speed input", runtimeOptions, async () => {
  const isolation = await createIsolatedProject();
  const result = await runGodot(isolation, "invalid");
  assertCompleted(result, 2);
  const measurement = measurementFrom(result);
  assert.equal(measurement.status, "error");
  assert.match(measurement.reason, /finite positive number/u);
  console.log(`Godot invalid-input measurement: ${JSON.stringify(measurement)}`);
});

after(async () => {
  for (const [name, expected] of fixtureSnapshot) {
    assert.equal(await readFile(path.join(fixtureRoot, name), "utf8"), expected, `${name} changed during tests`);
  }
  await assertMissing(path.join(fixtureRoot, ".godot"));
});
