import assert from "node:assert/strict";
import { getEventListeners } from "node:events";
import { mkdtemp, realpath, rename, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { Readable, Writable } from "node:stream";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import {
  FIXED_READ_TARGETS,
  MAX_ASSISTANT_PREVIEW_CHARS,
  NEGATIVE_FIXTURE,
  OPENING_TASK,
  PI_SDK_MODULE,
  RELEASE_TIMEOUT_MS,
  THINKING_LEVELS,
  ObservationBlocker,
  buildReadAllowlist,
  checkPhaseEvidence,
  createGuardExtension,
  createReadlineGate,
  createStageWaiter,
  evaluateToolCall,
  extractEventObservation,
  findToolSourceProblems,
  formatObservationStatus,
  inspectComposition,
  main,
  parseArgs,
  parseExplicitModelRef,
  parseModelInput,
  resolveExplicitModel,
  runObservation,
  settleWithin,
  terminalEventCode,
} from "./manual/ogs-readiness-observe.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const skillFile = path.join(repoRoot, "skills", "ogs-godot-change", "SKILL.md");
const LIVE_MODEL = "test-provider/test-model";
const DEFAULT_MODELS = [{ provider: "test-provider", id: "test-model", name: "Test Model" }];

const isBlocker = (code) => (error) => error instanceof ObservationBlocker && error.code === code;
const tick = (ms = 25) => new Promise((resolve) => setTimeout(resolve, ms));

/** Small SDK double: proves the delegation contract, not a runtime. */
function createSdkDouble(overrides = {}) {
  const calls = [];
  const prompts = [];
  const listeners = [];
  const models = overrides.models ?? DEFAULT_MODELS;
  let loaderOptions;
  let loaderInstance;
  let createOptions;
  let settingsOptions;
  // Invoke the observer's own guard extension so allowed attempts are recorded
  // exactly as they are in a live session (no fabricated model/settings data).
  const invokeGuard = async (event) => {
    const handlers = [];
    loaderOptions.extensionFactories[0]({ on: (name, handler) => handlers.push([name, handler]) });
    return handlers[0][1](event);
  };
  const session = {
    isStreaming: false,
    getAllTools: () => overrides.tools ?? [{ name: "read", sourceInfo: { source: "builtin" } }],
    subscribe(listener) {
      if (overrides.subscribeThrows) throw new Error("subscribe failed");
      calls.push("subscribe");
      listeners.push(listener);
      return () => {
        if (overrides.unsubscribeThrows) throw new Error("unsubscribe failed");
        calls.push("unsubscribe");
      };
    },
    async prompt(text, options) {
      calls.push("prompt");
      prompts.push(text);
      if (overrides.preflightRejected) {
        options?.preflightResult?.(false);
        throw new Error("preflight rejected");
      }
      options?.preflightResult?.(true);
      const synthetic = [];
      if (overrides.invokeGuard) {
        const decision = await invokeGuard(overrides.invokeGuard);
        if (decision?.block) throw new Error("blocked by guard");
      }
      if (overrides.emitDuringPrompt) {
        synthetic.push(...overrides.emitDuringPrompt);
      } else if (!overrides.invokeGuard && !overrides.suppressEvidence) {
        // Healthy phase: one allowed read attempt, its matching result and the
        // assistant text that a real phase always carries.
        const toolCallId = `call-${prompts.length}`;
        const decision = await invokeGuard({
          type: "tool_call",
          toolCallId,
          toolName: "read",
          input: { path: skillFile },
        });
        if (decision?.block) throw new Error("blocked by guard");
        synthetic.push(
          {
            type: "tool_execution_end",
            toolCallId,
            toolName: "read",
            isError: false,
            result: { content: [{ type: "text", text: "body" }] },
          },
          {
            type: "message_end",
            message: { role: "assistant", content: [{ type: "text", text: `phase ${prompts.length} complete` }] },
          },
        );
      }
      for (const event of synthetic) {
        for (const listener of listeners) listener(event);
      }
      if (overrides.promptThrows) throw new Error("prompt failed");
    },
    async waitForIdle() {
      calls.push("waitForIdle");
      if (overrides.waitForIdlePending) return new Promise(() => {});
      if (overrides.waitForIdleThrows) throw new Error("idle failed");
    },
    async abort() {
      if (overrides.abortPending) return new Promise(() => {});
      if (overrides.abortThrows) throw new Error("abort failed");
      calls.push("abort");
    },
    dispose() {
      if (overrides.disposeThrows) throw new Error("dispose failed");
      calls.push("dispose");
    },
  };
  const sdk = {
    getAgentDir: () => "/fake/agent-dir",
    ModelRuntime: {
      async create(options) {
        calls.push("ModelRuntime.create");
        if (overrides.modelRuntimeThrows) throw new Error("model runtime failed");
        if (overrides.modelRuntimePending) return new Promise(() => {});
        return { models, getModels: () => models, hasConfiguredAuth: () => true, options };
      },
    },
    resolveCliModel({ cliProvider, cliModel }) {
      calls.push("resolveCliModel");
      if (overrides.resolveCliModel) return overrides.resolveCliModel({ cliProvider, cliModel });
      let pattern = cliModel;
      let thinkingLevel;
      const colon = pattern.lastIndexOf(":");
      if (colon > 0 && THINKING_LEVELS.includes(pattern.slice(colon + 1))) {
        thinkingLevel = pattern.slice(colon + 1);
        pattern = pattern.slice(0, colon);
      }
      const match = models.find(
        (model) =>
          model.provider.toLowerCase() === String(cliProvider).toLowerCase() &&
          model.id.toLowerCase() === pattern.toLowerCase(),
      );
      if (!match) {
        return { model: undefined, thinkingLevel: undefined, warning: undefined, error: `Model "${pattern}" not found.` };
      }
      return { model: match, thinkingLevel, warning: undefined, error: undefined };
    },
    SettingsManager: {
      inMemory(settings) {
        settingsOptions = settings;
        return {};
      },
    },
    SessionManager: {
      inMemory(cwd) {
        calls.push("sessionManager.inMemory");
        return { kind: "session-in-memory", cwd };
      },
    },
    DefaultResourceLoader: class {
      constructor(options) {
        calls.push("loader.constructor");
        loaderOptions = options;
        loaderInstance = this;
      }
      async reload() {
        calls.push("loader.reload");
        if (overrides.reloadThrows) throw new Error("reload failed");
      }
      getExtensions() {
        return overrides.extensionsResult ?? { extensions: [{ path: "<inline:1>" }], errors: [] };
      }
      getSkills() {
        return overrides.skillsResult ?? { skills: [{ filePath: skillFile }], diagnostics: [] };
      }
      getPrompts() {
        return { prompts: [], diagnostics: [] };
      }
      getThemes() {
        return { themes: [], diagnostics: [] };
      }
    },
    async createAgentSession(options) {
      calls.push("createAgentSession");
      createOptions = options;
      if (overrides.createThrows) throw new Error("create failed");
      if (overrides.createSession) return overrides.createSession(options);
      if (overrides.createPending) return new Promise(() => {});
      return { session };
    },
  };
  return {
    sdk,
    calls,
    prompts,
    listeners,
    session,
    models,
    get loaderOptions() {
      return loaderOptions;
    },
    get loaderInstance() {
      return loaderInstance;
    },
    get createOptions() {
      return createOptions;
    },
    get settingsOptions() {
      return settingsOptions;
    },
  };
}

/** A double whose session is produced only after the run's deadline. */
function lateSessionDouble(dispose = () => {}) {
  return createSdkDouble({
    createSession: () =>
      new Promise((resolve) => {
        setTimeout(() => {
          resolve({
            session: {
              getAllTools: () => [{ name: "read", sourceInfo: { source: "builtin" } }],
              subscribe: () => () => {},
              prompt: async () => {},
              waitForIdle: async () => {},
              abort: async () => {},
              dispose,
            },
          });
        }, 120);
      }),
  });
}

function captureStdout() {
  const chunks = [];
  return { stream: { write: (chunk) => chunks.push(chunk) }, read: () => chunks.join("") };
}

/** In-memory readline inputs: the TTY seam is declared, not emulated. */
function ttyStream(text, extra = {}) {
  const stream = Readable.from(text === null ? [] : [text]);
  Object.assign(stream, { isTTY: true }, extra);
  return stream;
}

/** A stream that never ends, so the gate stays open until it is aborted. */
function openStream() {
  const stream = new Readable({ read() {} });
  stream.isTTY = true;
  return stream;
}

function sinkStream() {
  return new Writable({
    write(_chunk, _encoding, callback) {
      callback();
    },
  });
}

async function withTempDir(run) {
  const root = await mkdtemp(path.join(tmpdir(), "ogs-observe-"));
  try {
    return await run(root);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
}

test("default mode stays offline and never imports the SDK", async () => {
  let sdkImported = false;
  const out = captureStdout();
  const code = await main([], {
    env: {},
    stdout: out.stream,
    stderr: out.stream,
    importSdk: async () => {
      sdkImported = true;
      return {};
    },
  });
  assert.equal(code, 0);
  assert.equal(sdkImported, false);
  assert.match(out.read(), /Offline preparation/);
});

test("--help states preparation status, composition and limits", async () => {
  const out = captureStdout();
  const code = await main(["--help"], { env: {}, stdout: out.stream, stderr: out.stream });
  const text = out.read();
  assert.equal(code, 0);
  assert.match(text, /Live observation is pending/);
  assert.match(text, /NOT the full gentle-shell profile/);
  assert.match(text, /not human permission/i);
  assert.match(text, /OGS_DOGFOOD_LIVE/);
  assert.match(text, /model_unresolved/);
  assert.match(text, /SessionManager\.inMemory\(\) and SettingsManager\.inMemory\(\)/);
  assert.match(text, /--model <provider\/id\[:thinking\]>/);
  assert.match(text, /resolveCliModel/);
  assert.match(text, /model_ref_missing/);
  assert.match(text, /model_ambiguous/);
});

test("OGS_DOGFOOD_LIVE is ignored and never activates live mode", async () => {
  const parsed = parseArgs([], { OGS_DOGFOOD_LIVE: "1" });
  assert.equal(parsed.mode, "offline");
  assert.deepEqual(parsed.ignoredEnvFlags, ["OGS_DOGFOOD_LIVE"]);

  const out = captureStdout();
  const code = await main([], {
    env: { OGS_DOGFOOD_LIVE: "1" },
    stdout: out.stream,
    stderr: out.stream,
    importSdk: async () => {
      throw new Error("must not import");
    },
  });
  assert.equal(code, 0);
  assert.match(out.read(), /Ignored environment flags .*OGS_DOGFOOD_LIVE/);
});

test("removed and unknown options fail closed", () => {
  assert.equal(parseArgs(["--observe-live", "--workspace", repoRoot], {}).mode, "live");
  assert.equal(parseArgs(["--observe-live", "--allow-godot-probe"], {}).options.allowGodotAvailabilityProbe, true);
  assert.equal(parseArgs(["--observe-live", "--model", LIVE_MODEL], {}).options.model, LIVE_MODEL);
  for (const argv of [
    ["--task", "x"],
    ["--allow-read", "README.md"],
    ["--send-negative"],
    ["--ask-negative"],
    ["--no-negative"],
    ["--nope"],
    ["--deadline-ms", "0"],
    ["--model"],
    ["--model", "no-slash"],
    ["--model", "provider/"],
    ["--model", "/leading"],
  ]) {
    const parsed = parseArgs(argv, {});
    assert.equal(parsed.mode, "error", argv.join(" "));
  }
});

test("explicit model refs are parsed strictly", () => {
  assert.deepEqual(parseExplicitModelRef("anthropic/claude-opus-4-6"), {
    provider: "anthropic",
    modelId: "claude-opus-4-6",
    thinkingLevel: undefined,
  });
  assert.deepEqual(parseExplicitModelRef("anthropic/claude-opus-4-6:high"), {
    provider: "anthropic",
    modelId: "claude-opus-4-6",
    thinkingLevel: "high",
  });
  assert.deepEqual(parseExplicitModelRef("openrouter/moonshotai/kimi-k2.6"), {
    provider: "openrouter",
    modelId: "moonshotai/kimi-k2.6",
    thinkingLevel: undefined,
  });
  assert.deepEqual(parseExplicitModelRef("test-provider/test-model:bogus"), {
    provider: "test-provider",
    modelId: "test-model:bogus",
    thinkingLevel: undefined,
  });
  assert.equal(parseModelInput, parseExplicitModelRef);
  assert.equal(parseExplicitModelRef(undefined).error, "model_ref_missing");
  assert.equal(parseExplicitModelRef("   ").error, "model_ref_missing");
  for (const bad of ["no-slash", "/leading", "trailing/", "two words/model", "provider / id"]) {
    assert.equal(parseExplicitModelRef(bad).error, "model_ref_invalid", bad);
  }
});

test("resolveExplicitModel accepts only an exact, warning-free match", () => {
  const ref = parseExplicitModelRef(LIVE_MODEL);
  const exact = resolveExplicitModel(
    {
      resolveCliModel: () => ({
        model: { provider: "test-provider", id: "test-model" },
        thinkingLevel: undefined,
        warning: undefined,
      }),
    },
    {},
    ref,
  );
  assert.equal(exact.error, undefined);
  assert.equal(exact.model.id, "test-model");
  assert.equal(exact.thinkingLevel, undefined);
});

test("resolveExplicitModel fails closed on fuzzy, fabricated, warning and thrown results", () => {
  const ref = parseExplicitModelRef(LIVE_MODEL);
  const withResolution = (resolution) => resolveExplicitModel({ resolveCliModel: () => resolution }, {}, ref);

  assert.equal(withResolution({ error: "No models available." }).error, "model_unresolved");
  assert.equal(withResolution({ model: undefined }).error, "model_unresolved");
  assert.equal(withResolution({ model: { provider: "test-provider" } }).error, "model_unresolved");
  assert.equal(
    withResolution({
      model: { provider: "test-provider", id: "test-model" },
      warning: 'Model "test" not found for provider "test-provider". Using custom model id.',
    }).error,
    "model_ambiguous",
  );
  assert.equal(withResolution({ model: { provider: "test-provider", id: "test-model-2" } }).error, "model_ambiguous");
  assert.equal(withResolution({ model: { provider: "other-provider", id: "test-model" } }).error, "model_ambiguous");
  assert.equal(
    resolveExplicitModel(
      {
        resolveCliModel: () => {
          throw new Error("boom");
        },
      },
      {},
      ref,
    ).error,
    "model_unresolved",
  );
  assert.equal(resolveExplicitModel({}, {}, ref).error, "model_resolver_unavailable");

  const thinkingRef = parseExplicitModelRef(`${LIVE_MODEL}:high`);
  assert.equal(
    resolveExplicitModel(
      { resolveCliModel: () => ({ model: { provider: "test-provider", id: "test-model" }, thinkingLevel: "low" }) },
      {},
      thinkingRef,
    ).error,
    "model_ambiguous",
  );
  assert.equal(
    resolveExplicitModel(
      { resolveCliModel: () => ({ model: { provider: "test-provider", id: "test-model" }, thinkingLevel: "high" }) },
      {},
      thinkingRef,
    ).thinkingLevel,
    "high",
  );
});

test("settleWithin bounds a never-settling promise without rejecting", async () => {
  assert.equal(await settleWithin(Promise.resolve("ok"), 50), true);
  assert.equal(await settleWithin(new Promise(() => {}), 10), false);
  assert.equal(await settleWithin(Promise.reject(new Error("boom")), 50), false);
  assert.ok(Number.isFinite(RELEASE_TIMEOUT_MS) && RELEASE_TIMEOUT_MS > 0);
});

test("a stage waiter never starts queued work after cancellation", async () => {
  const controller = new AbortController();
  const waiter = createStageWaiter({ signal: controller.signal });
  let started = false;
  const pending = waiter.awaitStage("early", () => {
    started = true;
    return "value";
  });
  // Cancelled after the synchronous check but before the queued start runs.
  controller.abort();
  await assert.rejects(pending, isBlocker("aborted_early"));
  await tick(5);
  assert.equal(started, false);
});

test("a deadline that expires in the settlement gap fails the stage and reports the value once", async () => {
  const realNow = Date.now;
  let now = realNow();
  Date.now = () => now;
  try {
    const waiter = createStageWaiter({ deadlineAt: now + 60_000 });
    const late = [];
    const pending = waiter.awaitStage(
      "settle",
      () => {
        now += 120_000; // the deadline expires after the work already fulfilled
        return "value";
      },
      (entry) => late.push(entry),
    );
    await assert.rejects(pending, isBlocker("deadline_settle"));
    await tick(5);
    assert.deepEqual(late, [{ ok: true, value: "value" }]);
  } finally {
    Date.now = realNow;
  }
});

test("fixed read set resolves to canonical regular files", async () => {
  const allowlist = await buildReadAllowlist(repoRoot);
  assert.equal(allowlist.expected.size, FIXED_READ_TARGETS.length);
  assert.equal(await realpath(allowlist.root), await realpath(repoRoot));
  await assert.rejects(
    buildReadAllowlist(repoRoot, ["docs/does-not-exist.md"]),
    isBlocker("read_target_unresolved"),
  );
  await assert.rejects(buildReadAllowlist(repoRoot, ["skills"]), isBlocker("read_target_not_regular_file"));
});

test("read allowlist rejects symlinks, traversal and symlinked ancestors", async () => {
  await withTempDir(async (root) => {
    const outsideRoot = await mkdtemp(path.join(tmpdir(), "ogs-outside-"));
    try {
      const secret = path.join(outsideRoot, "secret.md");
      await writeFile(secret, "outside");

      await symlink(secret, path.join(root, "link.md"));
      await assert.rejects(buildReadAllowlist(root, ["link.md"]), isBlocker("read_target_symlink"));

      await symlink(outsideRoot, path.join(root, "sub"));
      await assert.rejects(
        buildReadAllowlist(root, [path.join("sub", "secret.md")]),
        isBlocker("read_target_identity_mismatch"),
      );

      await assert.rejects(buildReadAllowlist(root, ["../escape.md"]), isBlocker("read_target_not_confined"));
      await assert.rejects(buildReadAllowlist(root, [secret]), isBlocker("read_target_not_confined"));
      await assert.rejects(buildReadAllowlist(root, [""]), isBlocker("read_target_not_confined"));

      await writeFile(path.join(root, "note.md"), "note");
      const allowlist = await buildReadAllowlist(root, ["note.md"]);
      assert.equal(allowlist.expected.size, 1);
      assert.equal((await evaluateToolCall({ toolName: "read", input: { path: "note.md" }, allowlist })).allowed, true);
    } finally {
      await rm(outsideRoot, { recursive: true, force: true });
    }
  });
});

test("read guard fails closed when an approved file is replaced or aliased", async () => {
  await withTempDir(async (root) => {
    const note = path.join(root, "note.md");
    await writeFile(note, "original");
    const allowlist = await buildReadAllowlist(root, ["note.md"]);
    assert.equal((await evaluateToolCall({ toolName: "read", input: { path: "note.md" }, allowlist })).allowed, true);

    const replacement = path.join(root, "note.tmp");
    await writeFile(replacement, "replaced-with-a-different-size");
    await rename(replacement, note);
    const replaced = await evaluateToolCall({ toolName: "read", input: { path: "note.md" }, allowlist });
    assert.equal(replaced.allowed, false);
    assert.equal(replaced.reason, "read_target_replaced");

    await rm(note);
    await symlink(path.join(root, "elsewhere.md"), note);
    const aliased = await evaluateToolCall({ toolName: "read", input: { path: "note.md" }, allowlist });
    assert.equal(aliased.allowed, false);
    assert.equal(aliased.reason, "read_target_symlink");
  });
});

test("read guard allows only canonical identities", async () => {
  const allowlist = await buildReadAllowlist(repoRoot);
  const allow = await evaluateToolCall({ toolName: "read", input: { path: skillFile }, allowlist });
  assert.equal(allow.allowed, true);
  assert.equal(allow.target, path.join("skills", "ogs-godot-change", "SKILL.md"));

  const blocked = [
    { path: "skills/ogs-godot-change/references/other.md", reason: "read_target_not_allowlisted" },
    { path: "docs/consent-session-dogfood.md", reason: "read_target_not_allowlisted" },
    { path: "../../etc/hosts", reason: "read_path_traversal" },
    { path: "..\\etc\\hosts", reason: "read_path_traversal" },
    { path: "/etc/passwd", reason: "read_target_not_allowlisted" },
    { path: "", reason: "read_without_path" },
    { path: "README\u0000.md", reason: "read_path_invalid" },
  ];
  for (const { path: requested, reason } of blocked) {
    const decision = await evaluateToolCall({ toolName: "read", input: { path: requested }, allowlist });
    assert.equal(decision.allowed, false, JSON.stringify(requested));
    assert.equal(decision.reason, reason, JSON.stringify(requested));
  }

  assert.equal(
    await evaluateToolCall({ toolName: "read", input: { path: "README.md" }, allowlist }).then((d) => d.allowed),
    true,
  );
});

test("read guard rejects a changed canonical identity (symlink alias)", async () => {
  const aliasAllowlist = {
    root: repoRoot,
    expected: new Map([
      [path.resolve(repoRoot, "README.md"), { canonical: "/nonexistent/canonical", target: "README.md" }],
    ]),
  };
  const decision = await evaluateToolCall({ toolName: "read", input: { path: "README.md" }, allowlist: aliasAllowlist });
  assert.equal(decision.allowed, false);
  assert.equal(decision.reason, "read_target_identity_changed");
});

test("bash guard allows only the exact availability probes", async () => {
  const allowlist = await buildReadAllowlist(repoRoot);
  const base = { toolName: "bash", allowlist, allowGodotAvailabilityProbe: true };
  assert.equal((await evaluateToolCall({ ...base, input: { command: "command -v godot" } })).allowed, true);
  assert.equal((await evaluateToolCall({ ...base, input: { command: " command -v godot4 " } })).allowed, true);
  for (const command of [
    "command -v godot --version",
    "godot --headless --version",
    "command -v godot && rm -rf /",
    "command -v godot; curl http://example.com",
    "npm install -g godot",
    "",
  ]) {
    assert.equal((await evaluateToolCall({ ...base, input: { command } })).allowed, false, command);
  }
  assert.equal(
    (await evaluateToolCall({ toolName: "bash", allowlist, input: { command: "command -v godot" } })).allowed,
    false,
  );
});

test("non-read-only tools are always denied", async () => {
  const allowlist = await buildReadAllowlist(repoRoot);
  for (const toolName of ["write", "edit", "grep", "find", "ls", "task", "subagent_run", "mcp__x", "custom_tool"]) {
    const decision = await evaluateToolCall({ toolName, input: {}, allowlist, allowGodotAvailabilityProbe: true });
    assert.equal(decision.allowed, false, toolName);
    assert.equal(decision.reason, "tool_not_authorized");
  }
});

test("guard latches a denied attempt, keeps the tool call id and fails closed afterwards", async () => {
  const allowlist = await buildReadAllowlist(repoRoot);
  const records = [];
  let failed = false;
  const latches = [];
  let aborts = 0;
  const handlers = [];
  createGuardExtension({
    allowlist,
    record: (entry) => records.push(entry),
    isFailed: () => failed,
    failClosed: (code) => {
      failed = true;
      latches.push(code);
    },
    abortActiveRun: () => {
      aborts += 1;
    },
  })({ on: (event, handler) => handlers.push([event, handler]) });
  const handler = handlers[0][1];

  const allowed = await handler({ type: "tool_call", toolCallId: "c1", toolName: "read", input: { path: skillFile } });
  assert.equal(allowed, undefined);
  assert.deepEqual(records[0].toolCallId, "c1");
  assert.equal(records[0].decision, "allow");

  const denied = await handler({ type: "tool_call", toolCallId: "c2", toolName: "read", input: { path: "/etc/passwd" } });
  assert.equal(denied.block, true);
  assert.equal(denied.terminate, true);
  assert.equal(denied.reason, "read_target_not_allowlisted");
  assert.deepEqual(latches, ["unapproved_tool_attempt"]);
  assert.equal(aborts, 1);

  const afterLatch = await handler({ type: "tool_call", toolCallId: "c3", toolName: "read", input: { path: skillFile } });
  assert.equal(afterLatch.block, true);
  assert.equal(afterLatch.reason, "observation_already_failed");
  assert.equal(records.at(-1).toolCallId, "c3");
});

test("composition inspection fails closed on diagnostics and unexpected resources", async () => {
  const loader = {
    getExtensions: () => ({ extensions: [{ path: "<inline:1>" }], errors: [] }),
    getSkills: () => ({ skills: [{ filePath: skillFile }], diagnostics: [] }),
    getPrompts: () => ({ prompts: [], diagnostics: [] }),
    getThemes: () => ({ themes: [], diagnostics: [] }),
  };
  assert.deepEqual(inspectComposition(loader, skillFile), []);

  assert.deepEqual(
    inspectComposition(
      {
        ...loader,
        getExtensions: () => ({ extensions: [{ path: "<inline:1>" }], errors: [{ path: "x", error: "boom" }] }),
      },
      skillFile,
    ),
    ["extension_load_error"],
  );
  assert.deepEqual(
    inspectComposition(
      {
        ...loader,
        getExtensions: () => ({ extensions: [{ path: "<inline:1>" }, { path: "/tmp/evil.js" }], errors: [] }),
      },
      skillFile,
    ),
    ["unexpected_extension"],
  );
  assert.deepEqual(
    inspectComposition(
      { ...loader, getSkills: () => ({ skills: [], diagnostics: [{ type: "collision", message: "x" }] }) },
      skillFile,
    ),
    ["skills_diagnostic", "approved_skill_missing"],
  );
});

test("tool source inspection fails closed on a non-builtin override", () => {
  assert.deepEqual(findToolSourceProblems([{ name: "read", sourceInfo: { source: "builtin" } }], ["read"]), []);
  assert.deepEqual(findToolSourceProblems([{ name: "read", sourceInfo: { source: "user" } }], ["read"]), [
    "tool_source_not_builtin",
  ]);
  assert.deepEqual(
    findToolSourceProblems(
      [{ name: "read", sourceInfo: { source: "builtin" } }, { name: "mcp", sourceInfo: { source: "builtin" } }],
      ["read"],
    ),
    ["unexpected_tool"],
  );
  assert.deepEqual(findToolSourceProblems([], ["read", "bash"]), ["expected_tool_missing", "expected_tool_missing"]);
});

test("session events keep call ids, attempt/result distinction and truncation flags", () => {
  assert.deepEqual(
    extractEventObservation({
      type: "tool_execution_end",
      toolCallId: "c9",
      toolName: "read",
      isError: false,
      result: { content: [{ type: "text", text: "body" }] },
    }),
    {
      kind: "tool_result",
      toolCallId: "c9",
      toolName: "read",
      isError: false,
      resultChars: 4,
      resultPreview: "body",
      truncated: false,
    },
  );
  const longText = "x".repeat(5000);
  const assistant = extractEventObservation({
    type: "message_end",
    message: { role: "assistant", content: [{ type: "text", text: longText }] },
  });
  assert.equal(assistant.textChars, 5000);
  assert.equal(assistant.truncated, true);
  assert.equal(assistant.text.length, 4000);
  assert.equal(extractEventObservation({ type: "message_end", message: { role: "user", content: "hi" } }), null);
  assert.equal(extractEventObservation({ type: "session_start" }), null);
});

test("terminal event codes cover assistant and tool failures", () => {
  assert.equal(terminalEventCode({ type: "tool_execution_end", isError: true }), "tool_result_error");
  assert.equal(
    terminalEventCode({ type: "message_end", message: { role: "assistant", stopReason: "error" } }),
    "assistant_error",
  );
  assert.equal(
    terminalEventCode({ type: "message_end", message: { role: "assistant", stopReason: "aborted" } }),
    "assistant_aborted",
  );
  assert.equal(terminalEventCode({ type: "message_end", message: { role: "assistant", stopReason: "stop" } }), null);
});

test("phase evidence requires an assistant phase and correlated read proof", () => {
  const assistant = (phase, text = "done") => ({ kind: "assistant_text", phase, text });
  const attempt = (phase, toolCallId, over = {}) => ({
    kind: "tool_attempt",
    phase,
    toolCallId,
    toolName: "read",
    decision: "allow",
    ...over,
  });
  const result = (phase, toolCallId, over = {}) => ({
    kind: "tool_result",
    phase,
    toolCallId,
    toolName: "read",
    isError: false,
    ...over,
  });
  const validInitial = [assistant("initial"), attempt("initial", "c1"), result("initial", "c1")];

  assert.deepEqual(checkPhaseEvidence([]), {
    complete: false,
    reasons: ["initial_phase_empty", "negative_phase_empty"],
  });

  // Assistant prose is never proof that a file was read.
  assert.deepEqual(checkPhaseEvidence([assistant("initial"), assistant("negative")]).reasons, [
    "initial_read_evidence_missing",
  ]);

  // A correlated read pair without assistant text is not a complete phase.
  assert.deepEqual(
    checkPhaseEvidence([attempt("initial", "c1"), result("initial", "c1"), assistant("negative")]).reasons,
    ["initial_assistant_missing"],
  );

  // A tool-only phase one never counts, even with a fully matched pair.
  assert.deepEqual(checkPhaseEvidence([attempt("initial", "c1"), result("initial", "c1")]).reasons, [
    "initial_assistant_missing",
    "negative_phase_empty",
  ]);

  // A valid initial read pair plus a tool-free negative phase is complete.
  assert.deepEqual(checkPhaseEvidence([...validInitial, assistant("negative")]), { complete: true, reasons: [] });

  // The negative phase still needs its own assistant text.
  assert.deepEqual(checkPhaseEvidence([...validInitial, attempt("negative", "c2"), result("negative", "c2")]).reasons, [
    "negative_assistant_missing",
  ]);
});

test("phase evidence rejects orphan, ambiguous, mismatched, missing-id and failed records", () => {
  const assistant = (phase, text = "done") => ({ kind: "assistant_text", phase, text });
  const attempt = (phase, toolCallId, over = {}) => ({
    kind: "tool_attempt",
    phase,
    toolCallId,
    toolName: "read",
    decision: "allow",
    ...over,
  });
  const result = (phase, toolCallId, over = {}) => ({
    kind: "tool_result",
    phase,
    toolCallId,
    toolName: "read",
    isError: false,
    ...over,
  });
  const negative = [assistant("negative")];
  const validPair = [attempt("initial", "c1"), result("initial", "c1")];

  const cases = [
    { name: "orphan attempt", initial: [...validPair, attempt("initial", "c2")], reasons: ["initial_evidence_orphan"] },
    { name: "orphan result", initial: [...validPair, result("initial", "c3")], reasons: ["initial_evidence_orphan"] },
    {
      name: "ambiguous duplicate attempt id",
      initial: [...validPair, attempt("initial", "c1")],
      reasons: ["initial_evidence_ambiguous"],
    },
    {
      name: "mismatched result name",
      initial: [...validPair, attempt("initial", "c2"), result("initial", "c2", { toolName: "bash" })],
      reasons: ["initial_evidence_mismatched"],
    },
    { name: "missing id", initial: [...validPair, attempt("initial", null)], reasons: ["initial_evidence_missing_id"] },
    {
      name: "failed result",
      initial: [...validPair, attempt("initial", "c4"), result("initial", "c4", { isError: true })],
      reasons: ["initial_evidence_failed"],
    },
  ];
  for (const { name, initial, reasons } of cases) {
    const evidence = checkPhaseEvidence([assistant("initial"), ...initial, ...negative]);
    assert.equal(evidence.complete, false, name);
    assert.deepEqual(evidence.reasons, reasons, name);
  }

  // A good pair never masks an extra bad record.
  assert.deepEqual(
    checkPhaseEvidence([assistant("initial"), ...validPair, attempt("initial", "c9"), ...negative]).reasons,
    ["initial_evidence_orphan"],
  );

  // A blocked call is terminal, never read proof.
  assert.deepEqual(
    checkPhaseEvidence([
      assistant("initial"),
      ...validPair,
      attempt("initial", "c5", { decision: "block" }),
      result("initial", "c5"),
      ...negative,
    ]).reasons,
    ["initial_evidence_mismatched"],
  );
});

test("runObservation refuses to start without an explicit --model", async () => {
  let imported = false;
  await assert.rejects(
    runObservation(
      { cwd: repoRoot },
      {
        importSdk: async () => {
          imported = true;
          return {};
        },
      },
    ),
    isBlocker("model_ref_missing"),
  );
  assert.equal(imported, false);
});

test("runObservation rejects a pre-aborted signal before any SDK work", async () => {
  let imported = false;
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    runObservation(
      { cwd: repoRoot, signal: controller.signal },
      {
        importSdk: async () => {
          imported = true;
          return {};
        },
      },
    ),
    isBlocker("aborted_before_start"),
  );
  assert.equal(imported, false);
});

test("runObservation wires the narrowed public SDK composition", async () => {
  const double = createSdkDouble();
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
  );

  assert.deepEqual(double.settingsOptions, {
    retry: { enabled: false, maxRetries: 0 },
    compaction: { enabled: false },
    branchSummary: { skipPrompt: true },
  });
  assert.equal(double.loaderOptions.noExtensions, true);
  assert.equal(double.loaderOptions.noSkills, true);
  assert.equal(double.loaderOptions.noPromptTemplates, true);
  assert.equal(double.loaderOptions.noThemes, true);
  assert.equal(double.loaderOptions.noContextFiles, true);
  assert.equal(double.loaderOptions.systemPrompt, "");
  assert.deepEqual(double.loaderOptions.appendSystemPrompt, []);
  assert.deepEqual(double.loaderOptions.additionalSkillPaths, [path.join(repoRoot, "skills", "ogs-godot-change")]);
  assert.equal(double.loaderOptions.extensionFactories.length, 1);
  assert.equal(typeof double.loaderOptions.extensionFactories[0], "function");

  assert.deepEqual(double.createOptions.tools, ["read"]);
  assert.equal(double.createOptions.resourceLoader, double.loaderInstance);
  assert.deepEqual(double.createOptions.sessionManager, { kind: "session-in-memory", cwd: repoRoot });
  assert.equal(double.createOptions.model.id, "test-model");
  assert.equal(double.createOptions.thinkingLevel, undefined);
  assert.equal(double.createOptions.modelRuntime.models, double.models);
  assert.equal(double.calls.includes("ModelRuntime.create"), true);
  assert.equal(double.calls.includes("resolveCliModel"), true);
  assert.deepEqual(double.prompts, [OPENING_TASK, NEGATIVE_FIXTURE]);
  assert.equal(result.negativeSent, true);
  assert.equal(result.failure, null);
  assert.deepEqual(result.releaseFailures, []);
  assert.deepEqual(result.evidenceFailures, []);
  assert.deepEqual(double.calls.slice(-3), ["unsubscribe", "abort", "dispose"]);
  assert.match(formatObservationStatus(result), /^status: complete/m);
});

test("runObservation passes the exact resolved thinking level", async () => {
  const double = createSdkDouble();
  const result = await runObservation(
    { cwd: repoRoot, model: `${LIVE_MODEL}:high` },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => false },
  );
  assert.equal(result.failure, null);
  assert.equal(double.createOptions.thinkingLevel, "high");
  assert.equal(double.createOptions.model.id, "test-model");
});

test("runObservation fails closed when the resolver only fuzzy-matches", async () => {
  const double = createSdkDouble({
    resolveCliModel: () => ({
      model: { provider: "test-provider", id: "test-model" },
      thinkingLevel: undefined,
      warning: "Model not found. Using custom model id.",
    }),
  });
  const result = await runObservation(
    { cwd: repoRoot, model: "test-provider/test-mode" },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
  );
  assert.equal(result.failure.code, "model_ambiguous");
  assert.equal(double.calls.includes("createAgentSession"), false);
  assert.deepEqual(double.prompts, []);
});

test("probe mode exposes bash with the exact-command guard", async () => {
  const double = createSdkDouble({
    tools: [
      { name: "read", sourceInfo: { source: "builtin" } },
      { name: "bash", sourceInfo: { source: "builtin" } },
    ],
  });
  await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, allowGodotAvailabilityProbe: true },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => false },
  );
  assert.deepEqual(double.createOptions.tools, ["read", "bash"]);
});

test("runObservation stops after phase one when the operator declines", async () => {
  const double = createSdkDouble();
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => false },
  );
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.equal(result.negativeSent, false);
  assert.equal(result.failure, null);
  assert.match(formatObservationStatus(result), /reason=negative_not_sent/);
  assert.equal(formatObservationStatus(result).includes(NEGATIVE_FIXTURE), false);
});

test("a gate that cannot open stops before follow-up", async () => {
  const double = createSdkDouble();
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        throw new Error("no tty");
      },
    },
  );
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.equal(result.notes.includes("gate_unavailable"), true);
  assert.equal(result.negativeSent, false);
  assert.equal(result.failure, null);
});

test("cancellation during the manual gate stops the run", async () => {
  const controller = new AbortController();
  const double = createSdkDouble();
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, signal: controller.signal, deadlineMs: 5000 },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        controller.abort();
        return true;
      },
    },
  );
  assert.equal(result.failure.code, "aborted_during_gate");
  assert.equal(result.negativeSent, false);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.match(formatObservationStatus(result), /^status: failed-closed reason=aborted_during_gate/m);
});

test("an abort during preparation stops before the import and the session", async () => {
  const controller = new AbortController();
  let imported = false;
  const promise = runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, signal: controller.signal, deadlineMs: 5000 },
    {
      prepareAllowlist: () => new Promise(() => {}),
      importSdk: async () => {
        imported = true;
        return {};
      },
    },
  );
  setTimeout(() => controller.abort(), 10);
  const result = await promise;
  assert.equal(result.failure.code, "aborted_during_preparation");
  assert.equal(imported, false);
});

test("the overall deadline bounds a never-resolving idle wait", async () => {
  const double = createSdkDouble({ waitForIdlePending: true });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, deadlineMs: 80 },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
  );
  assert.equal(result.failure.code, "deadline_during_idle");
  assert.equal(result.negativeSent, false);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.equal(double.calls.includes("abort"), true);
  assert.equal(double.calls.includes("dispose"), true);
});

test("a late session is released through a bounded event without mutating the result", async () => {
  let disposed = 0;
  const double = lateSessionDouble(() => {
    disposed += 1;
  });
  const late = [];
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, deadlineMs: 40 },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => true,
      onRecord: (record) => late.push(record),
    },
  );
  assert.equal(result.failure.code, "deadline_before_session");
  const snapshot = structuredClone(result);
  await tick(200);
  assert.equal(disposed, 1);
  assert.deepEqual(late, [{ phase: "late", kind: "late_release", status: "released", step: "dispose" }]);
  assert.deepEqual(result, snapshot);
  assert.equal(result.notes.includes("late_session_disposed"), false);
  assert.deepEqual(result.releaseFailures, []);
});

test("a session created after the deadline is released exactly once and never reported sent", async () => {
  const realNow = Date.now;
  let now = realNow();
  Date.now = () => now;
  const releases = [];
  const late = [];
  try {
    const double = createSdkDouble({
      createSession: () => {
        now += 120_000; // the deadline expires once the session was produced
        return {
          session: {
            getAllTools: () => [{ name: "read", sourceInfo: { source: "builtin" } }],
            subscribe: () => () => double.prompts.push("unexpected.unsubscribe"),
            prompt: async () => double.prompts.push("unexpected.prompt"),
            waitForIdle: async () => {},
            abort: async () => releases.push("abort"),
            dispose: () => releases.push("dispose"),
          },
        };
      },
    });
    const result = await runObservation(
      { cwd: repoRoot, model: LIVE_MODEL, deadlineMs: 60_000 },
      {
        importSdk: async () => double.sdk,
        requestNegativeFixture: async () => true,
        onRecord: (record) => late.push(record),
      },
    );
    assert.equal(result.failure.code, "deadline_before_session");
    assert.equal(result.negativeSent, false);
    assert.deepEqual(result.releaseFailures, []);
    assert.deepEqual(double.prompts, []);
    const snapshot = structuredClone(result);
    await tick(5);
    assert.deepEqual(releases, ["dispose"]);
    assert.deepEqual(late, [{ phase: "late", kind: "late_release", status: "released", step: "dispose" }]);
    assert.deepEqual(result, snapshot);
  } finally {
    Date.now = realNow;
  }
});

test("a late disposal failure is reported instead of a clean note", async () => {
  let disposed = 0;
  const double = lateSessionDouble(() => {
    disposed += 1;
    throw new Error("dispose failed");
  });
  const late = [];
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, deadlineMs: 40 },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => true,
      onRecord: (record) => late.push(record),
    },
  );
  assert.equal(result.failure.code, "deadline_before_session");
  const snapshot = structuredClone(result);
  await tick(200);
  assert.equal(disposed, 1);
  assert.deepEqual(late, [{ phase: "late", kind: "late_release", status: "failed", step: "dispose" }]);
  assert.deepEqual(result, snapshot);
  assert.equal(result.notes.includes("late_session_dispose_failed"), false);
  assert.equal(result.releaseFailures.includes("dispose"), false);
  assert.equal(formatObservationStatus(result).includes("status: complete"), false);
  assert.equal(/dispose failed|Error/.test(formatObservationStatus(result)), false);
});

test("a throwing or rejecting late channel never escapes the observer", async () => {
  const unhandled = [];
  const onUnhandled = (reason) => unhandled.push(reason);
  process.on("unhandledRejection", onUnhandled);
  try {
    for (const onRecord of [
      () => {
        throw new Error("late channel failed");
      },
      () => Promise.reject(new Error("late channel rejected")),
    ]) {
      const double = lateSessionDouble();
      const result = await runObservation(
        { cwd: repoRoot, model: LIVE_MODEL, deadlineMs: 40 },
        { importSdk: async () => double.sdk, requestNegativeFixture: async () => true, onRecord },
      );
      assert.equal(result.failure.code, "deadline_before_session");
      await tick(200);
    }
    await tick(5);
    assert.deepEqual(unhandled.map(String), []);
  } finally {
    process.off("unhandledRejection", onUnhandled);
  }
});

test("a late gate rejection and a late session record never mutate the published result", async () => {
  const controller = new AbortController();
  let rejectGate;
  let gateEntered = false;
  const gate = new Promise((_, reject) => {
    rejectGate = reject;
  });
  const double = createSdkDouble({ disposeThrows: true });
  const pending = runObservation(
    { cwd: repoRoot, model: LIVE_MODEL, signal: controller.signal, deadlineMs: 5000 },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        gateEntered = true;
        return gate;
      },
    },
  );
  // Wait until the gate is genuinely pending, then stop the run while its
  // promise is still abandoned and unresolved.
  for (let i = 0; i < 200 && !gateEntered; i += 1) await tick(1);
  assert.equal(gateEntered, true);
  controller.abort();
  const result = await pending;
  assert.equal(result.failure.code, "aborted_during_gate");
  assert.equal(result.negativeSent, false);
  assert.deepEqual(result.releaseFailures, ["dispose"]);
  assert.deepEqual(result.evidenceFailures, ["negative_phase_empty"]);
  const snapshot = structuredClone(result);

  rejectGate(new Error("gate rejected after the run returned"));
  // A surviving session/guard callback is a double-driven stress probe, not a
  // normal-SDK guarantee: it must not reach the published snapshot either.
  for (const listener of double.listeners) {
    listener({ type: "message_end", message: { role: "assistant", content: [{ type: "text", text: "late" }] } });
  }
  await tick(5);
  assert.deepEqual(result, snapshot);
  assert.equal(result.notes.includes("gate_unavailable"), false);
});

test("a deadline that expires at the negative-idle settlement is never reported as sent", async () => {
  const realNow = Date.now;
  let now = realNow();
  Date.now = () => now;
  try {
    const double = createSdkDouble();
    const session = double.session;
    const realWaitForIdle = session.waitForIdle.bind(session);
    let idleCalls = 0;
    session.waitForIdle = async () => {
      idleCalls += 1;
      if (idleCalls === 2) now += 120_000;
      return realWaitForIdle();
    };
    const result = await runObservation(
      { cwd: repoRoot, model: LIVE_MODEL, deadlineMs: 60_000 },
      { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
    );
    assert.equal(result.failure.code, "deadline_during_idle");
    assert.equal(result.negativeSent, false);
    assert.match(formatObservationStatus(result), /^status: failed-closed reason=deadline_during_idle/m);
    assert.equal(double.calls.includes("dispose"), true);
  } finally {
    Date.now = realNow;
  }
});

test("a run whose initial phase has no evidence stops before the gate", async () => {
  let gateAsked = false;
  const double = createSdkDouble({ suppressEvidence: true });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        gateAsked = true;
        return true;
      },
    },
  );
  assert.equal(gateAsked, false);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.equal(result.failure, null);
  assert.equal(result.negativeSent, false);
  assert.deepEqual(result.evidenceFailures, ["initial_phase_empty"]);
  const status = formatObservationStatus(result);
  assert.match(status, /^status: incomplete reason=evidence_incomplete steps=initial_phase_empty/m);
  assert.equal(status.includes("status: complete"), false);
});

test("a tool-only phase one is never valid evidence", async () => {
  let gateAsked = false;
  const double = createSdkDouble({
    invokeGuard: { type: "tool_call", toolCallId: "c1", toolName: "read", input: { path: skillFile } },
    emitDuringPrompt: [
      {
        type: "tool_execution_end",
        toolCallId: "c1",
        toolName: "read",
        isError: false,
        result: { content: [{ type: "text", text: "body" }] },
      },
    ],
  });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        gateAsked = true;
        return true;
      },
    },
  );
  assert.equal(gateAsked, false);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.equal(result.negativeSent, false);
  assert.deepEqual(result.evidenceFailures, ["initial_assistant_missing"]);
  assert.match(formatObservationStatus(result), /reason=evidence_incomplete steps=initial_assistant_missing/);
});

test("an abort that never settles is reported as a partial release", async () => {
  const double = createSdkDouble({ abortPending: true });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => false, releaseTimeoutMs: 20 },
  );
  assert.deepEqual(result.releaseFailures, ["abort"]);
  assert.match(formatObservationStatus(result), /^status: incomplete reason=release_partial steps=abort/m);
});

test("truncated phase-one evidence latches incomplete before the gate", async () => {
  const longText = "x".repeat(MAX_ASSISTANT_PREVIEW_CHARS + 10);
  let gateAsked = false;
  const double = createSdkDouble({
    emitDuringPrompt: [
      { type: "message_end", message: { role: "assistant", content: [{ type: "text", text: longText }] } },
    ],
  });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        gateAsked = true;
        return true;
      },
    },
  );
  assert.equal(gateAsked, false);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
  assert.equal(result.truncated, true);
  const status = formatObservationStatus(result);
  assert.match(status, /^status: incomplete reason=evidence_truncated/m);
  assert.equal(status.includes("status: complete"), false);
});

test("formatObservationStatus never claims completion for truncated evidence", () => {
  const status = formatObservationStatus({
    failure: null,
    truncated: true,
    releaseFailures: [],
    notes: [],
    negativeSent: true,
  });
  assert.match(status, /^status: incomplete reason=evidence_truncated/m);
  assert.equal(status.includes("status: complete"), false);
});

test("readline gate accepts an explicit yes through a real interface", async () => {
  const input = ttyStream("yes\n");
  const result = await createReadlineGate({ input, output: sinkStream() });
  assert.deepEqual(result, { accepted: true, reason: "answer" });
  assert.equal(input.listenerCount("end"), 0);
});

test("readline gate treats no, EOF and a closed stream as refusal", async () => {
  assert.deepEqual(await createReadlineGate({ input: ttyStream("no\n"), output: sinkStream() }), {
    accepted: false,
    reason: "answer",
  });
  const eof = await createReadlineGate({ input: ttyStream(null), output: sinkStream() });
  assert.equal(eof.accepted, false);
  assert.ok(["eof", "closed"].includes(eof.reason), eof.reason);
  assert.deepEqual(await createReadlineGate({ input: { isTTY: false }, output: sinkStream() }), {
    accepted: false,
    reason: "not_tty",
  });
});

test("readline gate refuses a pre-aborted signal without opening an interface", async () => {
  const controller = new AbortController();
  controller.abort();
  let opened = false;
  const result = await createReadlineGate({
    input: ttyStream("yes\n"),
    output: sinkStream(),
    signal: controller.signal,
    createInterface: () => {
      opened = true;
      throw new Error("must not open");
    },
  });
  assert.deepEqual(result, { accepted: false, reason: "aborted" });
  assert.equal(opened, false);
});

test("readline gate resolves once on abort and removes its listeners", async () => {
  const controller = new AbortController();
  const input = openStream();
  const promise = createReadlineGate({ input, output: sinkStream(), signal: controller.signal });
  controller.abort();
  const result = await promise;
  assert.deepEqual(result, { accepted: false, reason: "aborted" });
  assert.equal(getEventListeners(controller.signal, "abort").length, 0);
  assert.equal(input.listenerCount("end"), 0);
});

test("readline gate reports an unavailable interface", async () => {
  const result = await createReadlineGate({
    input: ttyStream("yes\n"),
    output: sinkStream(),
    createInterface: () => {
      throw new Error("no terminal");
    },
  });
  assert.deepEqual(result, { accepted: false, reason: "unavailable" });
});

test("a denied tool attempt latches terminal failure and forbids follow-up", async () => {
  let gateAsked = false;
  const double = createSdkDouble({
    invokeGuard: { type: "tool_call", toolCallId: "c1", toolName: "bash", input: { command: "curl http://x" } },
  });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    {
      importSdk: async () => double.sdk,
      requestNegativeFixture: async () => {
        gateAsked = true;
        return true;
      },
    },
  );
  assert.equal(result.failure.code, "unapproved_tool_attempt");
  assert.equal(result.negativeSent, false);
  assert.equal(gateAsked, false);
  assert.equal(double.calls.includes("abort"), true);
  assert.match(formatObservationStatus(result), /^status: failed-closed reason=unapproved_tool_attempt/m);
});

test("an assistant error stop reason latches terminal failure", async () => {
  const double = createSdkDouble({
    emitDuringPrompt: [{ type: "message_end", message: { role: "assistant", stopReason: "error", content: [] } }],
  });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
  );
  assert.equal(result.failure.code, "assistant_error");
  assert.equal(result.negativeSent, false);
});

test("a subscription failure still releases the session", async () => {
  const double = createSdkDouble({ subscribeThrows: true });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
  );
  assert.equal(result.failure.code, "subscribe_failed");
  assert.deepEqual(double.prompts, []);
  assert.equal(double.calls.includes("abort"), true);
  assert.equal(double.calls.includes("dispose"), true);
});

test("release failures produce a sanitized incomplete status", async () => {
  const double = createSdkDouble({ unsubscribeThrows: true, abortThrows: true, disposeThrows: true });
  const result = await runObservation(
    { cwd: repoRoot, model: LIVE_MODEL },
    { importSdk: async () => double.sdk, requestNegativeFixture: async () => true },
  );
  assert.deepEqual(result.releaseFailures, ["unsubscribe", "abort", "dispose"]);
  const status = formatObservationStatus(result);
  assert.match(status, /^status: incomplete reason=release_partial steps=unsubscribe,abort,dispose/m);
  assert.equal(/Error|failed/.test(status), false);
});

test("main live path reports the model blocker without creating a session", async () => {
  const double = createSdkDouble();
  const out = captureStdout();
  const code = await main(["--observe-live", "--workspace", repoRoot], {
    env: {},
    stdout: out.stream,
    stderr: out.stream,
    stdin: { isTTY: false },
    importSdk: async (specifier) => {
      assert.equal(specifier, PI_SDK_MODULE);
      return double.sdk;
    },
  });
  assert.equal(code, 4);
  assert.equal(double.calls.includes("createAgentSession"), false);
  assert.match(out.read(), /observation stopped before session start: model_ref_missing/);
});

test("main live path prints a bounded status for a non-TTY run", async () => {
  const double = createSdkDouble();
  const out = captureStdout();
  const code = await main(["--observe-live", "--workspace", repoRoot, "--model", LIVE_MODEL], {
    env: {},
    stdout: out.stream,
    stderr: out.stream,
    stdin: { isTTY: false },
    importSdk: async () => double.sdk,
  });
  assert.equal(code, 3);
  const text = out.read();
  assert.match(text, /status: incomplete reason=negative_not_sent/);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
});

test("main reports incomplete evidence when the initial phase has no records", async () => {
  const double = createSdkDouble({ suppressEvidence: true });
  const out = captureStdout();
  let gateAsked = false;
  const code = await main(["--observe-live", "--workspace", repoRoot, "--model", LIVE_MODEL], {
    env: {},
    stdout: out.stream,
    stderr: out.stream,
    stdin: { isTTY: true },
    createGate: async () => {
      gateAsked = true;
      return { accepted: true, reason: "answer" };
    },
    importSdk: async () => double.sdk,
  });
  assert.equal(code, 3);
  assert.equal(gateAsked, false);
  assert.match(out.read(), /status: incomplete reason=evidence_incomplete/);
  assert.deepEqual(double.prompts, [OPENING_TASK]);
});

test("main prints a sanitized late failure line only after the run returned", async () => {
  let disposed = 0;
  const double = lateSessionDouble(() => {
    disposed += 1;
    throw new Error("dispose failed");
  });
  const out = captureStdout();
  const code = await main(
    ["--observe-live", "--workspace", repoRoot, "--model", LIVE_MODEL, "--deadline-ms", "60"],
    {
      env: {},
      stdout: out.stream,
      stderr: out.stream,
      stdin: { isTTY: false },
      importSdk: async () => double.sdk,
    },
  );
  assert.equal(code, 3);
  const beforeLate = out.read();
  assert.match(beforeLate, /status: failed-closed reason=(aborted_before_session|deadline_before_session)/);
  assert.equal(beforeLate.includes("late_session_release"), false);
  await tick(200);
  assert.equal(disposed, 1);
  const text = out.read();
  assert.match(text, /late_session_release: failed step=dispose/);
  assert.equal(text.includes("dispose failed"), false);
});
