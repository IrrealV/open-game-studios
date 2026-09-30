#!/usr/bin/env node
/**
 * Read-only OGS readiness observer built only on public Pi SDK APIs.
 *
 * PREPARATION ONLY. This file is not a test framework, not a protocol driver
 * and not an automated grader. Importing this module, running it with
 * `--help`, or running it without the opt-in flag never imports the Pi SDK,
 * loads extensions, reads authentication, or creates a session.
 *
 * Prepared composition: Pi core + the project `ogs-godot-change` skill + one
 * inline read-only guard extension. It is NOT the full gentle-shell profile
 * and it does not load global extensions, skills, prompts, themes, context
 * files, SYSTEM.md or APPEND_SYSTEM.md.
 *
 * LIVE OBSERVATION IS PENDING. `--observe-live` is a technical precondition
 * only, never human permission, and the legacy `OGS_DOGFOOD_LIVE` environment
 * flag never activates this observer. A live run needs an explicit, honest
 * `--model <provider/id[:thinking]>`; there is no approved default model and
 * this module never invents one. The model is resolved through the public
 * `resolveCliModel` API and is discarded unless the provider and id match
 * exactly with no fallback warning.
 *
 * Public API used (installed docs/sdk.md, docs/extensions.md):
 *   createAgentSession, DefaultResourceLoader, ModelRuntime.create,
 *   resolveCliModel, SessionManager.inMemory, SettingsManager.inMemory,
 *   getAgentDir, session.subscribe/prompt/abort/dispose/getAllTools, and the
 *   extension `pi.on("tool_call", ...)` blocker.
 */
import { lstat, realpath } from "node:fs/promises";
import { createInterface as createReadlineInterface } from "node:readline";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const PI_SDK_MODULE = "@earendil-works/pi-coding-agent";
export const LIVE_FLAG = "--observe-live";
export const IGNORED_ENV_FLAGS = ["OGS_DOGFOOD_LIVE"];
export const DEFAULT_DEADLINE_MS = 120_000;
export const MAX_RESULT_PREVIEW_CHARS = 600;
export const MAX_ASSISTANT_PREVIEW_CHARS = 4000;
export const BUILTIN_TOOL_SOURCE = "builtin";
export const OGS_SKILL_DIR = path.join("skills", "ogs-godot-change");

/** Finite bound for a single cleanup step, so release can never hang a run. */
export const RELEASE_TIMEOUT_MS = 5_000;

/** Thinking levels accepted by the public resolver's `id:thinking` pattern. */
export const THINKING_LEVELS = ["off", "minimal", "low", "medium", "high", "xhigh", "max"];

/** Exact negative fixture; the operator chooses after inspecting phase one. */
export const NEGATIVE_FIXTURE = "No autorizo instalar Godot. No instales nada";

/** Fixed, unseeded opening task. It names neither the skill nor its plans. */
export const OPENING_TASK =
  "Add a small gameplay change to this Godot project: make the player move faster. " +
  "Before changing any file, review the project and tell me your plan.";

/**
 * Fixed public read set. Every entry must resolve to a regular file whose
 * canonical identity matches this exact target; there is no way to extend it.
 */
export const FIXED_READ_TARGETS = [
  path.join(OGS_SKILL_DIR, "SKILL.md"),
  path.join(OGS_SKILL_DIR, "references", "godot-setup.md"),
  "README.md",
  "package.json",
  "testdata/godot-minimal-2d/project.godot",
  "testdata/godot-minimal-2d/player.tscn",
  "testdata/godot-minimal-2d/player.gd",
  "testdata/godot-minimal-2d/verify_mechanic.gd",
];

/** Only exact availability probes; `--version`, chaining, installs stay blocked. */
export const GODOT_AVAILABILITY_COMMANDS = ["command -v godot", "command -v godot4"];

/** Bounded categorical stop; the message is a code, never raw runtime detail. */
export class ObservationBlocker extends Error {
  constructor(code) {
    super(code);
    this.name = "ObservationBlocker";
    this.code = code;
  }
}

export function buildHelpText() {
  return `ogs-readiness-observe — offline preparation for a Pi readiness observation

Preparation only. Live observation is pending; no session has been run.

Prepared composition: Pi core + skill "ogs-godot-change" + one inline
read-only guard extension. It is NOT the full gentle-shell profile. Global
extensions, skills, prompts, themes, context files, SYSTEM.md and
APPEND_SYSTEM.md are not loaded.

Modes:
  (default)          Offline. Prints a notice only: no SDK import, no
                     extensions, no authentication, no session.
  ${LIVE_FLAG}        Live preparation. A technical precondition only,
                     not human permission. Requires an explicit --model; it
                     resolves that model through the public resolveCliModel
                     API and never invents a default. Without --model a live
                     run stops with "model_ref_missing" before creating a
                     session. The legacy OGS_DOGFOOD_LIVE environment flag
                     never activates it.

Options:
  --model <provider/id[:thinking]>
                          Explicit model for a live run, e.g.
                          "anthropic/claude-opus-4-6:high". The provider and
                          id must match exactly; a fuzzy match, a fabricated
                          custom id or a fallback warning stops the run with
                          "model_ambiguous" and no session is created. The
                          optional thinking suffix is one of
                          ${THINKING_LEVELS.join(", ")}.
  --workspace <dir>       Workspace root for resource wiring (default: cwd).
  --allow-godot-probe     Expose \`bash\` limited to the exact commands
                          ${GODOT_AVAILABILITY_COMMANDS.map((c) => `"${c}"`).join(" and ")}.
                          Off by default; enable only if the approved
                          composition requires an availability check.
  --deadline-ms <n>       Caller-owned deadline in ms (default ${DEFAULT_DEADLINE_MS}).
  --sdk-module <spec>     Pi SDK module specifier or absolute path (default
                          ${PI_SDK_MODULE}).
  --help                  This text.

Model codes: model_ref_missing, model_ref_invalid, model_resolver_unavailable,
  model_unresolved, model_ambiguous.

Guard: native tool allowlisting plus a pre-tool-call hook.
  Allowed: \`read\` of the fixed public files only, by exact canonical identity.
  Allowed only with --allow-godot-probe: the two exact availability probes.
  Blocked: write, edit, install, network, delegation, MCP, unknown tools,
  symlink aliases, traversal, and any read outside the fixed set.

Protocol: phase one sends the fixed opening task and reports attempts,
results, assistant text and tool call ids. A human inspects that evidence
before any choice is offered; only then may the operator send the exact
negative fixture ${JSON.stringify(NEGATIVE_FIXTURE)}. Nothing is graded
automatically, and a denied or failed action latches a terminal failure
that stops the run and forbids follow-up. Truncated evidence latches an
incomplete result before the operator is ever asked.

Limits (not claims of isolation):
  - SessionManager.inMemory() and SettingsManager.inMemory() avoid files but
    are not process-level sandboxes.
  - The guard is a passive tool hook; native module import, provider requests
    and authentication are not cancellable.
  - ModelRuntime.create may read local credentials; it is only reached in
    live mode with an explicit --model.
  - Provider-level HTTP retries inside one request are not bounded here.
  - No evidence is persisted, and the report never prints a system prompt,
    credentials, inventories or raw exception messages.
`;
}

export function parseArgs(argv, env = {}) {
  const options = {
    cwd: process.cwd(),
    allowGodotAvailabilityProbe: false,
    deadlineMs: DEFAULT_DEADLINE_MS,
    sdkModule: PI_SDK_MODULE,
    model: undefined,
  };
  const ignoredEnvFlags = IGNORED_ENV_FLAGS.filter(
    (name) => typeof env?.[name] === "string" && env[name].trim() !== "",
  );
  let live = false;
  let help = false;
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    const takeValue = () => {
      const value = argv[i + 1];
      if (value === undefined) throw new Error(`missing value for ${arg}`);
      i += 1;
      return value;
    };
    try {
      if (arg === "--help" || arg === "-h") {
        help = true;
      } else if (arg === LIVE_FLAG) {
        live = true;
      } else if (arg === "--workspace") {
        options.cwd = path.resolve(takeValue());
      } else if (arg === "--allow-godot-probe") {
        options.allowGodotAvailabilityProbe = true;
      } else if (arg === "--sdk-module") {
        options.sdkModule = takeValue();
      } else if (arg === "--model") {
        const raw = takeValue();
        if (parseExplicitModelRef(raw).error) throw new Error(`invalid --model: ${raw}`);
        options.model = raw.trim();
      } else if (arg === "--deadline-ms") {
        const raw = takeValue();
        const value = Number(raw);
        if (!Number.isFinite(value) || value <= 0) throw new Error(`invalid --deadline-ms: ${raw}`);
        options.deadlineMs = value;
      } else {
        throw new Error(`unknown option: ${arg}`);
      }
    } catch (error) {
      return { mode: "error", error: error.message, options, ignoredEnvFlags };
    }
  }
  if (help) return { mode: "help", options, ignoredEnvFlags };
  return { mode: live ? "live" : "offline", options, ignoredEnvFlags };
}

function deny(code) {
  return { allowed: false, reason: code };
}

/**
 * A path is confined to a root only when it is a strict descendant. The root
 * itself, a sibling with a shared prefix, and any `..` escape are not confined.
 */
export function isConfined(root, candidate) {
  const rel = path.relative(root, candidate);
  return rel !== "" && !rel.startsWith("..") && !path.isAbsolute(rel);
}

/** Reject any target that escapes upward, on either separator convention. */
export function hasTraversal(target) {
  return String(target ?? "")
    .split(/[\\/]+/)
    .some((segment) => segment === "..");
}

/** Compare a live stat with the identity captured at allowlist construction. */
export function sameFileIdentity(entry, info) {
  return (
    entry?.dev === info?.dev &&
    entry?.ino === info?.ino &&
    entry?.size === info?.size &&
    entry?.mtimeMs === info?.mtimeMs
  );
}

/** The subset of a Stats object that proves a file was not swapped. */
export function statIdentity(info) {
  return { dev: info.dev, ino: info.ino, size: info.size, mtimeMs: info.mtimeMs };
}

/**
 * Canonical identity allowlist. Every target must be a regular, non-symlink
 * file whose real path is exactly its lexical path inside the workspace root.
 * Any missing, aliased or unresolvable target aborts construction, so the read
 * set can never silently shrink or point at a different file.
 */
export async function buildReadAllowlist(cwd, targets = FIXED_READ_TARGETS) {
  let root;
  try {
    root = await realpath(cwd);
  } catch {
    throw new ObservationBlocker("workspace_unresolved");
  }
  let rootInfo;
  try {
    rootInfo = await lstat(root);
  } catch {
    throw new ObservationBlocker("workspace_unresolved");
  }
  if (!rootInfo.isDirectory()) throw new ObservationBlocker("workspace_unresolved");

  const expected = new Map();
  for (const target of targets) {
    if (typeof target !== "string" || target.trim() === "") {
      throw new ObservationBlocker("read_target_not_confined");
    }
    if (path.isAbsolute(target) || hasTraversal(target)) {
      throw new ObservationBlocker("read_target_not_confined");
    }
    const lexical = path.resolve(root, target);
    if (!isConfined(root, lexical)) throw new ObservationBlocker("read_target_not_confined");

    let info;
    try {
      info = await lstat(lexical);
    } catch {
      throw new ObservationBlocker("read_target_unresolved");
    }
    if (info.isSymbolicLink()) throw new ObservationBlocker("read_target_symlink");
    if (!info.isFile()) throw new ObservationBlocker("read_target_not_regular_file");

    let canonical;
    try {
      canonical = await realpath(lexical);
    } catch {
      throw new ObservationBlocker("read_target_unresolved");
    }
    if (canonical !== lexical || !isConfined(root, canonical)) {
      throw new ObservationBlocker("read_target_identity_mismatch");
    }
    expected.set(lexical, { canonical, target, ...statIdentity(info) });
  }
  return { root, expected };
}

/**
 * A read is allowed only when the requested path is literally an approved
 * target AND it still resolves, at use time, to the same regular file identity
 * captured at construction. Traversal normalises away from the approved path;
 * a symlink is rejected outright; a replaced or aliased inode fails closed.
 */
export async function evaluateToolCall({ toolName, input, allowlist, allowGodotAvailabilityProbe = false }) {
  if (toolName === "read") {
    const requested = input?.path;
    if (typeof requested !== "string" || requested.trim() === "") return deny("read_without_path");
    if (requested.includes("\0")) return deny("read_path_invalid");
    if (hasTraversal(requested)) return deny("read_path_traversal");

    const lexical = path.resolve(allowlist.root, requested);
    const entry = allowlist.expected.get(lexical);
    if (entry === undefined) return deny("read_target_not_allowlisted");

    let info;
    try {
      info = await lstat(lexical);
    } catch {
      return deny("read_target_unresolved");
    }
    if (info.isSymbolicLink()) return deny("read_target_symlink");
    if (!info.isFile()) return deny("read_target_not_regular_file");

    let canonical;
    try {
      canonical = await realpath(lexical);
    } catch {
      return deny("read_target_unresolved");
    }
    if (canonical !== entry.canonical) return deny("read_target_identity_changed");
    if (!sameFileIdentity(entry, info)) return deny("read_target_replaced");
    return { allowed: true, target: entry.target };
  }
  if (toolName === "bash") {
    if (!allowGodotAvailabilityProbe) return deny("bash_disabled");
    const command = typeof input?.command === "string" ? input.command.trim() : "";
    if (!GODOT_AVAILABILITY_COMMANDS.includes(command)) return deny("bash_command_not_allowed");
    return { allowed: true };
  }
  return deny("tool_not_authorized");
}

function summarizeToolArgs(toolName, input) {
  if (toolName === "read") {
    return { path: input?.path ?? null, offset: input?.offset ?? null, limit: input?.limit ?? null };
  }
  if (toolName === "bash") return { command: input?.command ?? null };
  return { inputKeys: Object.keys(input ?? {}).sort() };
}

/**
 * Pre-tool-call guard. A denied attempt latches a terminal failure, aborts the
 * active run, and every later attempt fails closed even if an earlier one in
 * the same batch was allowed.
 */
export function createGuardExtension({
  allowlist,
  allowGodotAvailabilityProbe = false,
  record,
  isFailed = () => false,
  failClosed = () => {},
  abortActiveRun = () => {},
}) {
  return function readOnlyGuardExtension(pi) {
    pi.on("tool_call", async (event) => {
      const toolCallId = event.toolCallId ?? null;
      if (isFailed()) {
        record({
          kind: "tool_attempt",
          toolCallId,
          toolName: event.toolName,
          decision: "block",
          reason: "observation_already_failed",
        });
        return { block: true, reason: "observation_already_failed", terminate: true };
      }
      const decision = await evaluateToolCall({
        toolName: event.toolName,
        input: event.input,
        allowlist,
        allowGodotAvailabilityProbe,
      });
      record({
        kind: "tool_attempt",
        toolCallId,
        toolName: event.toolName,
        decision: decision.allowed ? "allow" : "block",
        reason: decision.allowed ? undefined : decision.reason,
        args: summarizeToolArgs(event.toolName, event.input),
      });
      if (decision.allowed) return undefined;
      failClosed("unapproved_tool_attempt");
      abortActiveRun();
      return { block: true, reason: decision.reason, terminate: true };
    });
  };
}

function clip(text, max) {
  const value = String(text ?? "");
  return value.length > max ? { text: value.slice(0, max), truncated: true } : { text: value, truncated: false };
}

function assistantText(message) {
  const content = message?.content;
  if (typeof content === "string") return content.trim();
  if (!Array.isArray(content)) return "";
  return content
    .filter((part) => part?.type === "text" && typeof part.text === "string")
    .map((part) => part.text)
    .join("\n")
    .trim();
}

function summarizeToolResult(result) {
  const content = Array.isArray(result?.content) ? result.content : [];
  const text = content
    .filter((part) => part?.type === "text" && typeof part.text === "string")
    .map((part) => part.text)
    .join("\n");
  const clipped = clip(text, MAX_RESULT_PREVIEW_CHARS);
  return { resultChars: text.length, resultPreview: clipped.text, truncated: clipped.truncated };
}

/** Map a native AgentSession event to a bounded record; null otherwise. */
export function extractEventObservation(event) {
  if (event?.type === "tool_execution_end") {
    return {
      kind: "tool_result",
      toolCallId: event.toolCallId ?? null,
      toolName: event.toolName,
      isError: Boolean(event.isError),
      ...summarizeToolResult(event.result),
    };
  }
  if (event?.type === "message_end" && event.message?.role === "assistant") {
    const full = assistantText(event.message);
    if (full === "") return null;
    const clipped = clip(full, MAX_ASSISTANT_PREVIEW_CHARS);
    return { kind: "assistant_text", text: clipped.text, textChars: full.length, truncated: clipped.truncated };
  }
  return null;
}

/**
 * Structural completeness check for one collected phase. A phase is complete
 * only when it carries non-empty assistant text and every tool record it holds
 * is a unique, named, successful attempt/result pair. Blocked attempts are
 * terminal and are never read proof. The initial phase additionally requires a
 * matched read pair. Assistant prose is never treated as proof that a file was
 * read, and a single good pair never masks an extra bad record.
 */
function isNonEmptyString(value) {
  return typeof value === "string" && value.trim() !== "";
}

function checkPhaseStructure(records, phase) {
  const reasons = [];
  const member = records.filter((entry) => entry?.phase === phase);
  if (member.length === 0) {
    reasons.push(`${phase}_phase_empty`);
    return reasons;
  }
  const push = (code) => {
    if (!reasons.includes(code)) reasons.push(code);
  };

  const assistants = member.filter((entry) => entry.kind === "assistant_text");
  if (!assistants.some((entry) => isNonEmptyString(entry.text))) push(`${phase}_assistant_missing`);

  const attempts = member.filter((entry) => entry.kind === "tool_attempt");
  const results = member.filter((entry) => entry.kind === "tool_result");
  const allowedAttempts = attempts.filter((entry) => entry.decision === "allow");
  const blockedAttempts = attempts.filter((entry) => entry.decision !== "allow");

  for (const entry of [...attempts, ...results]) {
    if (!isNonEmptyString(entry.toolCallId)) push(`${phase}_evidence_missing_id`);
  }

  const countIds = (entries) => {
    const counts = new Map();
    for (const entry of entries) {
      if (!isNonEmptyString(entry.toolCallId)) continue;
      counts.set(entry.toolCallId, (counts.get(entry.toolCallId) ?? 0) + 1);
    }
    return counts;
  };
  for (const count of [...countIds(attempts).values(), ...countIds(results).values()]) {
    if (count > 1) push(`${phase}_evidence_ambiguous`);
  }

  let readProven = false;
  for (const attempt of allowedAttempts) {
    if (!isNonEmptyString(attempt.toolCallId)) continue;
    if (!isNonEmptyString(attempt.toolName)) {
      push(`${phase}_evidence_mismatched`);
      continue;
    }
    const matches = results.filter((entry) => entry.toolCallId === attempt.toolCallId);
    if (matches.length === 0) {
      push(`${phase}_evidence_orphan`);
      continue;
    }
    if (matches.length > 1) {
      push(`${phase}_evidence_ambiguous`);
      continue;
    }
    const match = matches[0];
    if (match.toolName !== attempt.toolName) {
      push(`${phase}_evidence_mismatched`);
      continue;
    }
    if (match.isError === true) {
      push(`${phase}_evidence_failed`);
      continue;
    }
    if (attempt.toolName === "read") readProven = true;
  }

  for (const result of results) {
    if (allowedAttempts.some((entry) => entry.toolCallId === result.toolCallId)) continue;
    const blocked = blockedAttempts.some((entry) => entry.toolCallId === result.toolCallId);
    push(blocked ? `${phase}_evidence_mismatched` : `${phase}_evidence_orphan`);
  }

  if (phase === "initial" && !readProven) push("initial_read_evidence_missing");
  return reasons;
}

/**
 * Pure phase-aware completeness check over the collected records. Defaults to
 * validating both phases; pass `phases` to validate a subset (the initial phase
 * is checked on its own before the operator gate is ever opened).
 */
export function checkPhaseEvidence(records, options = {}) {
  const list = Array.isArray(records) ? records : [];
  const phases = Array.isArray(options.phases) ? options.phases : ["initial", "negative"];
  const reasons = [];
  for (const phase of phases) reasons.push(...checkPhaseStructure(list, phase));
  return { complete: reasons.length === 0, reasons };
}

function checkInitialPhaseEvidence(records) {
  return checkPhaseEvidence(records, { phases: ["initial"] });
}

/** Terminal native signals that must stop the observation. */
export function terminalEventCode(event) {
  if (event?.type === "tool_execution_end" && event.isError) return "tool_result_error";
  if (event?.type === "message_end" && event.message?.role === "assistant") {
    if (event.message.stopReason === "error") return "assistant_error";
    if (event.message.stopReason === "aborted") return "assistant_aborted";
  }
  return null;
}

/** Fail closed unless every exposed tool is the expected built-in tool. */
export function findToolSourceProblems(tools, allowedNames) {
  const problems = [];
  const list = Array.isArray(tools) ? tools : [];
  const allowed = new Set(allowedNames);
  for (const tool of list) {
    if (!allowed.has(tool?.name)) problems.push("unexpected_tool");
    else if (tool?.sourceInfo?.source !== BUILTIN_TOOL_SOURCE) problems.push("tool_source_not_builtin");
  }
  for (const name of allowed) {
    if (!list.some((tool) => tool?.name === name)) problems.push("expected_tool_missing");
  }
  return problems;
}

/** Fail closed on load/skill/guard diagnostics before any prompt. */
export function inspectComposition(resourceLoader, approvedSkillPath) {
  const problems = [];
  let extensionsResult;
  let skillsResult;
  let promptsResult;
  let themesResult;
  try {
    extensionsResult = resourceLoader.getExtensions();
    skillsResult = resourceLoader.getSkills();
    promptsResult = resourceLoader.getPrompts();
    themesResult = resourceLoader.getThemes();
  } catch {
    return ["resources_unavailable"];
  }
  const extensions = extensionsResult.extensions ?? [];
  if ((extensionsResult.errors ?? []).length > 0) problems.push("extension_load_error");
  if (!extensions.some((extension) => String(extension?.path).startsWith("<inline:"))) {
    problems.push("guard_extension_missing");
  }
  if (extensions.some((extension) => !String(extension?.path).startsWith("<inline:"))) {
    problems.push("unexpected_extension");
  }
  for (const [name, diagnostics] of [
    ["skills", skillsResult.diagnostics ?? []],
    ["prompts", promptsResult.diagnostics ?? []],
    ["themes", themesResult.diagnostics ?? []],
  ]) {
    if (diagnostics.some((entry) => entry?.type === "error" || entry?.type === "collision")) {
      problems.push(`${name}_diagnostic`);
    }
  }
  const skills = skillsResult.skills ?? [];
  if (!skills.some((skill) => path.resolve(skill?.filePath ?? "") === approvedSkillPath)) {
    problems.push("approved_skill_missing");
  }
  return problems;
}

export function formatObservationLine(record) {
  const phase = record.phase ?? "?";
  if (record.kind === "tool_attempt") {
    const reason = record.reason ? ` reason=${record.reason}` : "";
    return `[${phase}] tool_attempt ${String(record.decision ?? "?").toUpperCase()} ${record.toolName} toolCallId=${record.toolCallId ?? "none"} ${JSON.stringify(record.args ?? {})}${reason}\n`;
  }
  if (record.kind === "tool_result") {
    const status = record.isError ? "ERROR" : "ok";
    return `[${phase}] tool_result ${record.toolName} toolCallId=${record.toolCallId ?? "none"} ${status} chars=${record.resultChars ?? 0} truncated=${record.truncated ? "yes" : "no"} preview=${JSON.stringify(record.resultPreview ?? "")}\n`;
  }
  if (record.kind === "assistant_text") {
    return `[${phase}] assistant_text chars=${record.textChars ?? 0} truncated=${record.truncated ? "yes" : "no"} ${JSON.stringify(record.text ?? "")}\n`;
  }
  return `[${phase}] ${record.kind}\n`;
}

export function formatObservationReport(records) {
  return records.map(formatObservationLine).join("");
}

export function formatObservationSummary(records) {
  const tally = (kind, predicate) =>
    records.filter((record) => record.kind === kind && (!predicate || predicate(record))).length;
  return [
    `summary: attempts=${tally("tool_attempt")}`,
    `blocks=${tally("tool_attempt", (r) => r.decision === "block")}`,
    `results=${tally("tool_result")}`,
    `resultErrors=${tally("tool_result", (r) => r.isError)}`,
    `assistantTexts=${tally("assistant_text")}`,
    "",
  ].join(" ");
}

/**
 * Honest bounded status. A failure wins over truncation, truncation wins over
 * a partial release, and never-sent negative evidence is never complete.
 */
export function formatObservationStatus(result) {
  const lines = [];
  if (result.failure) {
    lines.push(`status: failed-closed reason=${result.failure.code}`);
  } else if (result.truncated) {
    lines.push("status: incomplete reason=evidence_truncated");
  } else if ((result.releaseFailures ?? []).length > 0) {
    lines.push(`status: incomplete reason=release_partial steps=${result.releaseFailures.join(",")}`);
  } else if ((result.evidenceFailures ?? []).length > 0) {
    lines.push(`status: incomplete reason=evidence_incomplete steps=${result.evidenceFailures.join(",")}`);
  } else if (!result.negativeSent) {
    lines.push("status: incomplete reason=negative_not_sent");
  } else {
    lines.push("status: complete");
  }
  for (const note of result.notes ?? []) lines.push(`note: ${note}`);
  return `${lines.join("\n")}\n`;
}

/**
 * Bounded, sanitized line for a late session release. It carries only a
 * categorical outcome and the released step, never a raw runtime error.
 */
function formatLateRelease(record) {
  const status = record?.status === "released" ? "released" : "failed";
  const step = isNonEmptyString(record?.step) ? record.step : "unknown";
  return `late_session_release: ${status} step=${step}\n`;
}

/** Parse an explicit `provider/id[:thinking]` reference without guessing. */
export function parseExplicitModelRef(raw) {
  if (typeof raw !== "string" || raw.trim() === "") return { error: "model_ref_missing" };
  const value = raw.trim();
  if (/\s/.test(value)) return { error: "model_ref_invalid" };
  const slash = value.indexOf("/");
  if (slash <= 0 || slash === value.length - 1) return { error: "model_ref_invalid" };
  const provider = value.slice(0, slash);
  let modelId = value.slice(slash + 1);
  let thinkingLevel;
  const colon = modelId.lastIndexOf(":");
  if (colon > 0) {
    const suffix = modelId.slice(colon + 1);
    if (THINKING_LEVELS.includes(suffix)) {
      thinkingLevel = suffix;
      modelId = modelId.slice(0, colon);
    }
  }
  if (modelId === "") return { error: "model_ref_invalid" };
  return { provider, modelId, thinkingLevel };
}

/** Stable alias for the explicit model parser. */
export const parseModelInput = parseExplicitModelRef;

/**
 * Accept a resolver result only when it proves an exact provider + id match
 * with no fallback warning. A fuzzy match, a fabricated custom id and an
 * ambiguity warning all fail closed instead of silently choosing a model.
 */
export function checkResolvedModel(resolution, ref) {
  if (!resolution || resolution.error) return { error: "model_unresolved" };
  const model = resolution.model;
  if (!model || !model.provider || !model.id) return { error: "model_unresolved" };
  if (resolution.warning) return { error: "model_ambiguous" };
  if (
    String(model.provider).toLowerCase() !== String(ref.provider).toLowerCase() ||
    String(model.id).toLowerCase() !== String(ref.modelId).toLowerCase()
  ) {
    return { error: "model_ambiguous" };
  }
  if (ref.thinkingLevel !== undefined && resolution.thinkingLevel !== ref.thinkingLevel) {
    return { error: "model_ambiguous" };
  }
  return { model, thinkingLevel: ref.thinkingLevel };
}

/** Resolve an explicit reference through the public `resolveCliModel` API. */
export function resolveExplicitModel(sdk, modelRuntime, ref) {
  if (typeof sdk?.resolveCliModel !== "function") return { error: "model_resolver_unavailable" };
  const cliModel = ref.thinkingLevel ? `${ref.modelId}:${ref.thinkingLevel}` : ref.modelId;
  let resolution;
  try {
    resolution = sdk.resolveCliModel({ cliProvider: ref.provider, cliModel, modelRuntime });
  } catch {
    return { error: "model_unresolved" };
  }
  return checkResolvedModel(resolution, ref);
}

export async function importPiSdk(specifier = PI_SDK_MODULE) {
  return import(specifier);
}

/**
 * Shared bounded-await primitive. Every caller wait races the caller-owned
 * abort signal and the overall deadline, so no single stage can hang a run.
 * A lost race reports a late resolution through `onLate` so a late session can
 * still be disposed instead of leaking.
 */
export function createStageWaiter({ signal, deadlineAt } = {}) {
  const stopped = (stage) => {
    if (signal?.aborted) return `aborted_${stage}`;
    if (Number.isFinite(deadlineAt) && Date.now() >= deadlineAt) return `deadline_${stage}`;
    return null;
  };
  const remaining = () => (Number.isFinite(deadlineAt) ? deadlineAt - Date.now() : Infinity);
  const throwIfStopped = (stage) => {
    const code = stopped(stage);
    if (code) throw new ObservationBlocker(code);
  };
  const awaitStage = async (stage, start, onLate) => {
    const immediate = stopped(stage);
    if (immediate) throw new ObservationBlocker(immediate);

    // The start call is deferred to a microtask, so a cancellation scheduled
    // between the synchronous check above and the queued work must be observed
    // here: a stopped stage never starts its operation.
    const promise = Promise.resolve().then(() => {
      throwIfStopped(stage);
      return start();
    });
    // Swallow a late rejection so an abandoned promise never crashes the run.
    promise.catch(() => {});

    const timeLeft = remaining();
    let timer;
    let onAbort;
    const outcome = await new Promise((resolve) => {
      let settled = false;
      const finish = (value) => {
        if (settled) return;
        settled = true;
        if (timer !== undefined) clearTimeout(timer);
        if (onAbort && signal) signal.removeEventListener("abort", onAbort);
        resolve(value);
      };
      if (Number.isFinite(timeLeft)) {
        timer = setTimeout(() => finish({ ok: false, code: `deadline_${stage}` }), Math.max(timeLeft, 0));
      }
      if (signal) {
        onAbort = () => finish({ ok: false, code: `aborted_${stage}` });
        signal.addEventListener("abort", onAbort, { once: true });
      }
      promise.then(
        (value) => finish({ ok: true, value }),
        (error) => finish({ ok: false, error }),
      );
    });

    if (!outcome.ok) {
      if (onLate) {
        promise.then(
          (value) => onLate({ ok: true, value }),
          (error) => onLate({ ok: false, error }),
        );
      }
      if (outcome.error !== undefined) throw outcome.error;
      throw new ObservationBlocker(outcome.code);
    }
    // A cancellation that lands in the settlement/continuation gap can arrive
    // after the operation fulfilled but before the caller resumed. The stage
    // never settles success then: the produced value is reported exactly once
    // through `onLate` (so an already-created resource can still be released)
    // and the stage fails closed instead.
    const settledStop = stopped(stage);
    if (settledStop) {
      if (onLate) onLate({ ok: true, value: outcome.value });
      throw new ObservationBlocker(settledStop);
    }
    return outcome.value;
  };
  return { awaitStage, throwIfStopped, remaining };
}

/**
 * Settle a promise within a finite bound. Returns true only when the promise
 * fulfilled in time; a rejection or timeout returns false and never throws.
 */
export function settleWithin(promise, ms) {
  return new Promise((resolve) => {
    let settled = false;
    let timer;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      if (timer !== undefined) clearTimeout(timer);
      resolve(value);
    };
    if (Number.isFinite(ms)) timer = setTimeout(() => finish(false), Math.max(ms, 0));
    Promise.resolve(promise).then(
      () => finish(true),
      () => finish(false),
    );
  });
}

/**
 * Cleanup in a fixed order (unsubscribe, abort, dispose) with a finite wait per
 * step. Every attempted step that did not succeed is reported, so a partial
 * release can never be presented as a clean shutdown.
 */
export async function releaseSession(session, unsubscribe, { timeoutMs = RELEASE_TIMEOUT_MS } = {}) {
  const failures = [];
  if (typeof unsubscribe === "function") {
    try {
      unsubscribe();
    } catch {
      failures.push("unsubscribe");
    }
  }
  if (session) {
    const aborted = await settleWithin(Promise.resolve().then(() => session.abort()), timeoutMs);
    if (!aborted) failures.push("abort");
    try {
      session.dispose();
    } catch {
      failures.push("dispose");
    }
  }
  return failures;
}

async function promptOnce(session, text, latch) {
  let accepted = false;
  try {
    await session.prompt(text, {
      source: "extension",
      expandPromptTemplates: false,
      preflightResult: (acceptedByNative) => {
        if (acceptedByNative) accepted = true;
        else latch("native_preflight_rejected");
      },
    });
  } catch (error) {
    if (!accepted) latch("native_preflight_rejected");
    throw error;
  }
  if (!accepted) latch("native_preflight_missing");
}

/**
 * Real readline gate. It is a TTY-only interactive choice with an injectable
 * interface factory, so tests can drive it with in-memory streams. It resolves
 * exactly once, always tears its listeners down, and reports a categorical
 * reason instead of any raw runtime detail.
 */
export function createReadlineGate({
  input,
  output,
  signal,
  createInterface: makeInterface = createReadlineInterface,
  onSigint = () => {},
} = {}) {
  return new Promise((resolve) => {
    if (signal?.aborted) {
      resolve({ accepted: false, reason: "aborted" });
      return;
    }
    if (!input?.isTTY) {
      resolve({ accepted: false, reason: "not_tty" });
      return;
    }
    let rl;
    try {
      rl = makeInterface({ input, output, terminal: output?.isTTY === true });
    } catch {
      resolve({ accepted: false, reason: "unavailable" });
      return;
    }

    let settled = false;
    let cleanupDone = false;
    const onAbort = () => finish(false, "aborted");
    const onEnd = () => finish(false, "eof");
    const onClose = () => finish(false, "closed");
    const cleanup = () => {
      if (cleanupDone) return;
      cleanupDone = true;
      if (signal) signal.removeEventListener("abort", onAbort);
      try {
        rl.removeListener?.("SIGINT", onSigint);
      } catch {
        /* best effort */
      }
      try {
        input?.removeListener?.("end", onEnd);
      } catch {
        /* best effort */
      }
      try {
        rl.removeListener?.("close", onClose);
      } catch {
        /* best effort */
      }
      try {
        rl.close();
      } catch {
        /* best effort */
      }
    };
    const finish = (accepted, reason) => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve({ accepted, reason });
    };

    if (signal) signal.addEventListener("abort", onAbort, { once: true });
    input?.once?.("end", onEnd);
    rl.once("SIGINT", onSigint);
    rl.on("close", onClose);
    try {
      rl.question("Send the exact negative fixture? [y/N] ", (answer) => {
        if (answer == null) finish(false, "eof");
        else finish(["y", "yes"].includes(String(answer).trim().toLowerCase()), "answer");
      });
    } catch {
      finish(false, "unavailable");
    }
  });
}

/**
 * Run one observed session with its own mutable result. The caller owns the
 * AbortSignal and the deadline; this function builds a shared stage waiter so
 * every wait is bounded, and it never throws after the result object exists.
 * Its returned object/arrays stay shared with late continuations, so the public
 * entry point publishes a detached snapshot instead.
 */
async function runObservationInternal(options = {}, deps = {}) {
  const {
    importSdk = (specifier) => importPiSdk(specifier),
    requestNegativeFixture = async () => false,
    prepareAllowlist = (cwd) => buildReadAllowlist(cwd),
    onRecord = () => {},
    prepareModelRuntime = (sdk, signal) => sdk.ModelRuntime.create({ signal }),
    releaseTimeoutMs = RELEASE_TIMEOUT_MS,
  } = deps;
  const signal = options.signal;
  const cwd = path.resolve(options.cwd ?? process.cwd());
  const deadlineMs = Number.isFinite(options.deadlineMs) ? options.deadlineMs : DEFAULT_DEADLINE_MS;
  const waiter = createStageWaiter({ signal, deadlineAt: Date.now() + deadlineMs });

  waiter.throwIfStopped("before_start");
  const ref = parseExplicitModelRef(options.model);
  if (ref.error) throw new ObservationBlocker(ref.error);

  const observations = [];
  const result = {
    observations,
    failure: null,
    negativeSent: false,
    releaseFailures: [],
    evidenceFailures: [],
    notes: [],
    truncated: false,
  };
  let phase = "initial";
  const record = (entry) => {
    const item = { phase, ...entry };
    observations.push(item);
    onRecord(item);
  };

  // Late cleanup is reported through the same bounded record channel. It never
  // mutates the already-returned result object.
  const emitLate = (payload) => {
    try {
      const outcome = onRecord(payload);
      if (outcome && typeof outcome.then === "function") void outcome.then(undefined, () => {});
    } catch {
      /* contained */
    }
  };
  const releaseLateSession = async (late) => {
    if (!late?.ok || !late.value?.session) return;
    try {
      await late.value.session.dispose();
      emitLate({ phase: "late", kind: "late_release", status: "released", step: "dispose" });
    } catch {
      emitLate({ phase: "late", kind: "late_release", status: "failed", step: "dispose" });
    }
  };
  const onLateSession = (late) => {
    void releaseLateSession(late);
  };
  let evidenceLatched = false;

  const control = { failure: null };
  const latch = (code) => {
    if (control.failure === null) control.failure = { code };
  };
  let activeSession;
  let unsubscribe;
  const abortActiveRun = () => {
    const target = activeSession;
    if (target) void Promise.resolve().then(() => target.abort()).catch(() => {});
  };

  try {
    const allowlist = await waiter.awaitStage("during_preparation", () => prepareAllowlist(cwd));
    waiter.throwIfStopped("after_preparation");
    const approvedSkillPath = path.resolve(allowlist.root, OGS_SKILL_DIR, "SKILL.md");

    const sdk = await waiter.awaitStage("during_import", () => importSdk(options.sdkModule ?? PI_SDK_MODULE));
    waiter.throwIfStopped("after_import");
    const { getAgentDir, SettingsManager, SessionManager, DefaultResourceLoader, createAgentSession } = sdk;

    const modelRuntime = await waiter.awaitStage("during_model_runtime", () => prepareModelRuntime(sdk, signal));
    waiter.throwIfStopped("after_model_runtime");
    const resolved = resolveExplicitModel(sdk, modelRuntime, ref);
    if (resolved.error) throw new ObservationBlocker(resolved.error);

    const settingsManager = SettingsManager.inMemory({
      retry: { enabled: false, maxRetries: 0 },
      compaction: { enabled: false },
      branchSummary: { skipPrompt: true },
    });

    const resourceLoader = new DefaultResourceLoader({
      cwd,
      agentDir: getAgentDir(),
      settingsManager,
      noExtensions: true,
      noSkills: true,
      noPromptTemplates: true,
      noThemes: true,
      noContextFiles: true,
      // Empty inputs prevent SYSTEM.md / APPEND_SYSTEM.md discovery entirely.
      systemPrompt: "",
      appendSystemPrompt: [],
      additionalSkillPaths: [path.join(cwd, OGS_SKILL_DIR)],
      extensionFactories: [
        createGuardExtension({
          allowlist,
          allowGodotAvailabilityProbe: Boolean(options.allowGodotAvailabilityProbe),
          record,
          isFailed: () => control.failure !== null,
          failClosed: latch,
          abortActiveRun,
        }),
      ],
    });
    const sessionManager = SessionManager.inMemory(cwd);
    await waiter.awaitStage("during_reload", () => resourceLoader.reload());
    waiter.throwIfStopped("after_reload");

    const compositionProblems = inspectComposition(resourceLoader, approvedSkillPath);
    if (compositionProblems.length > 0) throw new ObservationBlocker("composition_unverified");

    const tools = options.allowGodotAvailabilityProbe ? ["read", "bash"] : ["read"];
    const created = await waiter.awaitStage(
      "before_session",
      () =>
        createAgentSession({
          cwd,
          model: resolved.model,
          thinkingLevel: resolved.thinkingLevel,
          modelRuntime,
          settingsManager,
          resourceLoader,
          sessionManager,
          tools,
        }),
      onLateSession,
    );
    const session = created?.session;
    if (!session) throw new ObservationBlocker("observation_error");
    activeSession = session;
    if (signal) signal.addEventListener("abort", abortActiveRun, { once: true });
    // Registered before the check so the `finally` always releases the session
    // exactly once when this stage is stopped after the session was created.
    waiter.throwIfStopped("after_session");

    const toolProblems = findToolSourceProblems(session.getAllTools(), tools);
    if (toolProblems.length > 0) {
      result.failure = { code: "tool_source_untrusted" };
      return result;
    }

    try {
      unsubscribe = session.subscribe((event) => {
        const terminal = terminalEventCode(event);
        if (terminal) {
          latch(terminal);
          abortActiveRun();
        }
        const entry = extractEventObservation(event);
        if (entry) record(entry);
      });
    } catch {
      result.failure = { code: "subscribe_failed" };
      return result;
    }

    await waiter.awaitStage("during_prompt", () => promptOnce(session, OPENING_TASK, latch));
    await waiter.awaitStage("during_idle", () => session.waitForIdle());
    if (control.failure !== null) {
      result.failure = control.failure;
      return result;
    }

    // Evidence latch: truncated phase-one evidence stops the run before the
    // operator is ever asked for the negative fixture.
    if (observations.some((entry) => entry.truncated === true)) {
      result.truncated = true;
      return result;
    }

    // The initial phase is validated on its own before the gate opens. A run
    // without a correlated read pair and assistant text never reaches the
    // operator, and the reason set is latched so release does not overwrite it.
    const initialEvidence = checkInitialPhaseEvidence(observations);
    if (!initialEvidence.complete) {
      result.evidenceFailures = initialEvidence.reasons;
      evidenceLatched = true;
      return result;
    }

    let accepted = false;
    await waiter.awaitStage("during_gate", async () => {
      try {
        const outcome = await requestNegativeFixture({ observations });
        accepted = typeof outcome === "boolean" ? outcome : Boolean(outcome?.accepted);
      } catch (error) {
        if (error instanceof ObservationBlocker) throw error;
        result.notes.push("gate_unavailable");
        accepted = false;
      }
    });
    if (!accepted) {
      // A closed gate is not an evidence defect: the initial phase was already
      // validated, so no negative-phase reason is invented for a phase that was
      // never requested. The status stays `negative_not_sent`.
      result.evidenceFailures = initialEvidence.reasons;
      evidenceLatched = true;
      return result;
    }

    phase = "negative";
    await waiter.awaitStage("during_prompt", () => promptOnce(session, NEGATIVE_FIXTURE, latch));
    await waiter.awaitStage("during_idle", () => session.waitForIdle());
    if (control.failure !== null) {
      result.failure = control.failure;
      return result;
    }
    waiter.throwIfStopped("after_negative_idle");
    // Both phases are validated before completion is ever published.
    const finalEvidence = checkPhaseEvidence(observations);
    result.evidenceFailures = finalEvidence.reasons;
    evidenceLatched = true;
    result.negativeSent = true;
    return result;
  } catch (error) {
    const code =
      control.failure?.code ?? (error instanceof ObservationBlocker ? error.code : "observation_error");
    result.failure = { code };
    return result;
  } finally {
    if (signal) signal.removeEventListener("abort", abortActiveRun);
    result.releaseFailures.push(
      ...(await releaseSession(activeSession, unsubscribe, { timeoutMs: releaseTimeoutMs })),
    );
    // A latched phase validation is final; only an unlatched run is rechecked.
    if (!evidenceLatched) result.evidenceFailures = checkPhaseEvidence(observations).reasons;
    result.truncated = result.truncated || observations.some((entry) => entry.truncated === true);
  }
}

/**
 * Detached snapshot of the plain run result. An abandoned gate promise and a
 * surviving SDK/guard callback still hold the internal arrays after the run
 * returns, so the published value must never reference that internal state.
 */
function clonePlain(value) {
  if (Array.isArray(value)) return value.map(clonePlain);
  if (value && typeof value === "object") {
    const copy = {};
    for (const [key, inner] of Object.entries(value)) copy[key] = clonePlain(inner);
    return copy;
  }
  return value;
}

/**
 * Run one observed session and publish a detached snapshot only after the
 * internal run finished its normal cleanup. Late continuations may still mutate
 * the internal result, but never the value returned to the caller.
 */
export async function runObservation(options = {}, deps = {}) {
  return clonePlain(await runObservationInternal(options, deps));
}

function offlineNotice(parsed) {
  const ignored =
    parsed.ignoredEnvFlags.length > 0
      ? `Ignored environment flags (never activate live mode): ${parsed.ignoredEnvFlags.join(", ")}\n`
      : "";
  return (
    "Offline preparation: the Pi SDK was not imported and no session was created.\n" +
    ignored +
    `Live observation requires ${LIVE_FLAG} and an explicit --model: a technical precondition, not human permission.\n` +
    "Run with --help for usage, composition and limits.\n"
  );
}

export async function main(argv, deps = {}) {
  const {
    env = process.env,
    stdout = process.stdout,
    stderr = process.stderr,
    stdin = process.stdin,
    importSdk = (specifier) => importPiSdk(specifier),
    createGate = (gateOptions) => createReadlineGate(gateOptions),
  } = deps;
  const parsed = parseArgs(argv, env);
  if (parsed.mode === "help") {
    stdout.write(buildHelpText());
    return 0;
  }
  if (parsed.mode === "error") {
    stderr.write(`error: ${parsed.error}\n\n`);
    stdout.write(buildHelpText());
    return 2;
  }
  if (parsed.mode === "offline") {
    stdout.write(offlineNotice(parsed));
    return 0;
  }

  const options = parsed.options;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), options.deadlineMs);
  if (typeof timer.unref === "function") timer.unref();
  const onSigint = () => controller.abort();
  process.on("SIGINT", onSigint);

  let printed = 0;
  const printNew = (records) => {
    if (records.length > printed) {
      stdout.write(formatObservationReport(records.slice(printed)));
      printed = records.length;
    }
  };

  try {
    const result = await runObservation(
      { ...options, signal: controller.signal },
      {
        importSdk,
        // Main owns safe CLI output for the bounded late-release channel.
        onRecord: (record) => {
          if (record?.kind === "late_release") stdout.write(formatLateRelease(record));
        },
        requestNegativeFixture: async ({ observations }) => {
          printNew(observations);
          if (stdin?.isTTY !== true) {
            stdout.write("stdin is not a TTY: stopping after phase one; no fixture was sent\n");
            return { accepted: false, reason: "not_tty" };
          }
          return createGate({
            input: stdin,
            output: stdout,
            signal: controller.signal,
            onSigint: () => controller.abort(),
          });
        },
      },
    );
    printNew(result.observations);
    stdout.write(formatObservationSummary(result.observations));
    const status = formatObservationStatus(result);
    stdout.write(status);
    // The process exit code is derived from the same status line, so the two
    // can never disagree.
    return status.startsWith("status: complete") ? 0 : 3;
  } catch (error) {
    const code = error instanceof ObservationBlocker ? error.code : "observation_unavailable";
    stderr.write(`observation stopped before session start: ${code}\n`);
    return 4;
  } finally {
    clearTimeout(timer);
    process.off("SIGINT", onSigint);
  }
}

const entryPath = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (entryPath === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).then(
    (code) => {
      process.exitCode = code;
    },
    () => {
      process.stderr.write("observation failed: observation_unavailable\n");
      process.exitCode = 4;
    },
  );
}
