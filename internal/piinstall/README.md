# internal/piinstall

Bounded Go backend that detects Pi-only prerequisites, proposes immutable
conditional actions, requires explicit human approval, executes the approved
plan, and reports the observed per-component result. It is the callable core the
retained wizard connects (G4 backend with G5a/G5b wiring and G5c parity); it is
not a wizard, orchestrator, plugin framework, or generic dependency registry.

## Caller API

```go
cfg := piinstall.Config{
    WorkspaceDir:  workspace,   // receives .pi/skills/ogs-godot-change
    InstallRoot:   installRoot, // owned per-user root for managed installs
    GodotRequired: godotNeeded,
    // Optional explicit executable paths (recommended when a tool is installed
    // outside PATH): NodePath, NpmPath, PiPath, EngramPath, GodotPath.
    Commands:   nil, // production CommandRunner (ExecRunner)
    Downloader: nil, // production Downloader (HTTPDownloader)
}

detection, plan, err := piinstall.Prepare(ctx, cfg) // read-only, zero writes
// show plan.Steps (Outputs and Effects) to the human
consent := piinstall.Consent{Approved: true, PlanFingerprint: plan.Fingerprint()}
report, err := piinstall.Execute(ctx, cfg, plan, consent)
```

- `Detect` / `Prepare` run only read-only version probes. They never run a
  mutating command and never run `pi list`, because package commands initialize
  Pi bootstrap settings.
- `BuildPlan` turns a `Detection` into ordered `PlanStep`s with `Action` values
  `reuse`, `install`, `ensure`, or `blocked`, each carrying the planned
  `Outputs` and disclosed `Effects`.
- `Consent.PlanFingerprint` binds the full structured plan (the public
  `GodotRequired` flag, every step's action, reason, command, outputs, effects,
  and the full observed state including its component) together with the
  normalized execution config (workspace, install root, Godot requirement, and
  explicit executable paths). `Execute` recomputes only the normalized binding
  from the current config while keeping the approved displayed fields, and
  refuses with `ErrPlanDrift` when the current config or any displayed field
  differs from the approved digest, before any write or installer command. The
  same pure normalization is applied by `Config.withDefaults`, which `Detect`,
  `BuildPlan`, and `Execute` all call, so every path used by validation and
  filesystem/command IO is the same lexical path that was approved; an unset
  optional path stays empty instead of collapsing to the current directory.
- `Execute` requires `Consent.Approved` and a matching
  `Consent.PlanFingerprint`; a zero-value `Consent` is never approved. It
  re-probes before writing and fails closed when the approved plan is stale. It
  stops on the first failure and preserves completed work; it does not roll back
  the machine.
- `Report.Launch` returns the explicit launch environment (Pi executable and
  prefix, Node bin dir, npm, `ENGRAM_BIN`, `GODOT_BIN`, PATH additions) for a
  later real launch. The installer never edits the user's PATH, shell profile,
  or any global configuration.
- `InstallRoot` must be an owned per-user destination. Shared or system
  prefixes (`/`, `/usr`, `/etc`, `/var`, `/opt`, bare `/home` or `/Users`, the
  exact shared temp roots `/tmp` and `/var/tmp`, and Windows system trees) are
  rejected with `ErrUnsafeRoot`, and a private per-user directory beneath a
  known temp root remains valid. In addition to the string policy, `Prepare` and
  `Execute` validate local filesystem metadata along the root and its existing
  ancestors: an existing managed root must be owned by the current user and must
  not be group/world-writable; a symlinked or non-directory component is
  rejected. Root-owned non-writable ancestors (and root-owned sticky shared temp
  roots) are legitimate; a foreign non-root, non-current-owned ancestor is
  rejected regardless of its mode, because that owner controls its children even
  when they are not group/world-writable. The temp roots are a fixed platform
  list, not `os.TempDir()`, so a caller-configured `TMPDIR` (for example `/usr`)
  can never bypass the system-prefix protection. When a component is missing, the
  nearest existing creation parent must be owned by the current user or be a
  narrowly trusted root-owned sticky shared temp base; a missing component under
  a root-owned non-sticky parent such as `/home` is refused instead of created
  optimistically. The walk is seeded with the filesystem root, so a missing first
  component has a concrete creation parent to validate: the filesystem root is
  never an acceptable creation parent, even for a caller running as UID 0, and
  empty or nil creation-parent metadata fails closed. The created root is
  revalidated after creation before any managed probe or write. This is a
  metadata policy check, not a sandbox: it never scans host configuration, never
  resolves symlinks, never chowns or chmods anything, and makes no race-free
  claim.

Managed runtime env: every managed child process (the Pi launcher's
`--version` probe, `pi list`/`pi install`, and `npm install`) runs with the
managed Node bin directory prepended to `PATH`, so a Node-backed launcher works
on a fresh machine even when the managed Node is not on the host PATH.

The managed Node layout is `node/<version>/bin/node` (with `npm`/`npx` as
in-root symlinks). Detection, install, reuse, and reporting all use the same
layout, and a second setup reuses the managed Node/npm without redownloading or
reinstalling.

When a compatible Node exists but npm is missing, acquiring the pinned official
Node bundle as an npm provider is separated from selecting the Node runtime: the
selected Node stays the existing compatible Node (`Report.Launch.NodeBinDir`
still points at it), and the provider npm is probed under that runtime. Pi's own
package manager spawns `npm` by `PATH` when it installs a package, so Pi's child
`PATH` exposes the provider npm directory, but only after the selected Node bin
directory. Both invariants therefore hold at once: the chosen Node still wins
`node` lookup, and `npm` is reachable for a later `pi install npm:...`. The plan
continues to disclose that the bundle files are acquired for npm only, and the
existing Node is left byte-for-byte unchanged. The npm provider and the Pi
binary are probed under the managed Node environment before any success is
reported; a missing or non-runnable npm is a failure, never a success. No new
npm URL, version, or checksum is invented.

The only exported seams are `CommandRunner` and `Downloader`; tests replace them
with fakes and a loopback HTTPS server. A single unexported `Config` field lets
in-package tests substitute local archive fixtures for the pinned digests.

## Pinned candidate facts

The executable candidate catalog is defined in `catalog.go`. These are
published candidates, **not certified integration evidence**: archive contents,
transitive npm lifecycle effects, and runtime interoperability remain unverified
until a separate consented consumer acceptance runs.

| Component | Candidate | Source kind |
| --- | --- | --- |
| Node / npm | 24.21.0 / 11.19.0 | official Linux x64 tar.gz + pinned SHA-256 (archive size not pinned) |
| Pi | `@earendil-works/pi-coding-agent@0.87.1` | npm, `--global --prefix <owned> --ignore-scripts` |
| Gentle Shell | `gentle-pi@3.7.0` | ordinary `pi install ... --no-approve` |
| Engram core | 2.2.0 | official tar.gz + pinned SHA-256 and size |
| Engram companion | `gentle-engram@0.1.15` | ordinary `pi install ... --no-approve` |
| Godot (optional) | 4.7.2 | official zip + pinned SHA-256 and size |

## Version policy

A component is reused only when its probe output parses as a version at or above
the minimum in `minimumVersion` (Node 22.19.0, npm 10.0.0, Pi 0.87.1, Engram
2.2.0, Godot 4.x). An observed but unparseable version is `unknown`, and a lower
version is `incompatible`; both make the plan `blocked` rather than silently
upgrading, downgrading, or replacing an existing executable.

Lookups use `PATH` plus caller-supplied explicit paths. **PATH absence is not
proof that a tool is absent from the whole machine**; when a tool is installed
elsewhere, pass its explicit path in `Config`.

The `pi list` classifier fails closed. A section header with no records, a
`No packages installed.` marker shown alongside section headers, a record before
any recognized header, an unrecognized/indented row, a filtered row, cross-scope
or duplicate registrations, and conflicting versions all classify as
`unverified` or `conflict` rather than being treated as an active or absent
registration.

The public Pi listing format prints a recognized source at two spaces and, under
it, the installed path at four (see `dist/package-manager-cli.js`, the `list`
case: `  ${display}` and `    ${pkg.installedPath}`). The classifier uses that
layout rather than the indentation-insensitive path test it replaced: only a
two-space row can be a record, and a four-space path row is accepted only as the
immediate producer detail line beneath a recognized record and is never scanned
for the target. Any other two-space row, including an opaque local path source
such as `./local-package`, is an unrecognized source row and classifies as
`unverified`; it never falls through to a path detail or proves the target
absent.

Git records are recognized only in a documented source shape: an HTTPS/SSH URL,
an scp-style `git@host:namespace/repo`, or the dotted-host/`localhost`
shorthand, and every recognized form is validated by one shared conservative
repository-path/ref contract grounded in Pi's public parser
(`dist/utils/git.js`). Processing order is fixed: separate the repository path
from URL userinfo and the SCP host, split the FIRST repository `@ref` (a slash
inside the ref never counts toward repository depth), reject unsafe raw or
encoded forms before any normalization, strip a terminal `.git` from the
repository portion, then require at least two meaningful namespace/repository
segments (each nonempty and not `.`, `..`, or `.git`). Both the raw and the
final repository portion are validated. Unsafe forms (`\`, NUL, a leading
slash, a `..` segment or its percent-encoded equivalent, an undecodable escape,
or a percent-encoded sequence that is not valid UTF-8 such as `%FF` or the
surrogate `%ED%A0%80` — which Pi's `decodeURIComponent` rejects but Go's
`url.PathUnescape` would return as raw bytes — and an `.git`-only repository)
and one-component repositories such as
`https://example.com/user`, `example.com/user`, or `git@example.com:user` stay
`unverified` instead of being cleaned into an accepted source.

The URL subset is HTTPS (the URL row in `docs/packages.md`,
`https://github.com/example/pi-tools`, "Treated as a git source") and SSH (the
`pi install ssh://git@github.com/user/repo` example in
`dist/package-manager-cli.js`), prefixed with exactly lowercase `https://` or
`ssh://`. Pi's own parser also accepts `http://` and `git://`; this installer
deliberately keeps those schemes, an uppercase protocol such as `HTTPS://`, and
any other hierarchical scheme `unverified` rather than broadening protocol
support, so the policy describes this backend's recognition subset and does not
claim Pi supports no others. `net/url.Parse` is not the WHATWG `new URL`: it
validates that a port is numeric but not that it is in range, and it accepts
bracketed IPv6, percent-encoded/zone, and numeric-resolved hosts whose producer
acceptance is unproven. The conservative host subset therefore rejects
bracketed/IPv6 (`https://[::1]/user/repo`), percent-encoded/zone, an out-of-range
or zero port, and — without emulating WHATWG's IPv4 algorithm — any host whose
final label is all digits or begins with `0x`/`0X`. A trailing host dot is
ignored when identifying that final label, so `https://999.999.999.999/owner/repo`
and `https://192.168.0.1/owner/repo` stay `unverified`. This common subset
intentionally also excludes valid dotted-decimal IPv4 and hexadecimal/numeric-
ended names while preserving ordinary DNS names, `localhost`, and single-label
SSH aliases such as `build01`. It requires a hierarchical URL with a real
hostname and at least two repository segments. A
malformed or unsupported URL-like value such as `https:/`, `https:`, `https://`,
`https://:443/user/repo` (port-only authority), or `https://user@/user/repo`
(userinfo without a hostname) never falls through to the shorthand/scp
alternative, and the ambiguous `git@github.com/user/repo` (`user@host/slash`,
where the canonical SCP form uses `:`) stays `unverified`. A bare `git:`, a
whitespace-only payload, or an opaque/unsupported git or local source classifies
as `unverified` instead of certifying absence. The parser performs no network
lookup, adds no dependency, and makes no remote-existence or DNS claim.

npm records are accepted only as `npm:<name>@<plain semver>` where the name is a
validated registry name (an unscoped lowercase segment or an `@scope/segment`
pair) and the version is a plain semver triple. A bare `@` prefix (`npm:@@1.0.0`),
an empty scope, a missing version, ranges/aliases, and other unsupported specs
stay `unverified`; this is a small validated subset and is not a claim that any
rejected spec is universally invalid for npm. The classifier follows Pi's
current public listing output, so a future format change fails closed rather than
mis-reporting.

## Consent, side effects, and limits

- Pi and the Engram companion are ordinary **personal-scope** Pi packages
  installed with `pi install <source> --no-approve`. They are never installed
  with a standalone launcher or through `gentle-ai install`/sync/TUI.
- Pi package commands initialize Pi bootstrap settings, and Gentle Shell's
  postinstall manages its package-private native runtime and can persist
  personal `tuiMode: fullscreen`. The plan discloses this before consent.
- Engram core returns `ENGRAM_BIN` and a PATH addition for later real
  write-through. Installing the package is not MCP setup; no Engram call, MCP
  adapter, model configuration, or gameplay execution is performed.
- Archive downloads are HTTPS-only, bounded by context, and verified against the
  pinned SHA-256 (and size when pinned) before extraction. HTTPS is enforced on
  every redirect hop as well as the initial request, without mutating a
  caller-supplied `http.Client`; a supplied client's own redirect policy is
  still honored, but the 10-hop bound is enforced before it so a permissive
  callback cannot bypass it. SHA-256 protects the bytes, while the HTTPS rule is
  the separate transport restriction. Extraction rejects absolute paths, `..`
  traversal, drive prefixes, escaping or absolute symlinks, duplicate
  collisions, and symlinked destination ancestors/leaves. Node's legitimate
  in-root `npm`/`npx` symlinks are deferred until regular files are written and
  are accepted only when they stay inside the extraction root.
- Existing payload files are read only after refusing to follow a symlink at the
  payload leaf or any ancestor below the workspace. A file that is byte-identical
  is reported `identical`; a differing file is preserved and reported
  `preserved`. If every embedded file already exists but differs, nothing is
  installed, the outcome is `unverified`, and the report is partial rather than
  claiming a reuse. A partial mix installs the missing files and truthfully
  reports the preserved ones.
- Cleanup removes only the staging directory this call created. Completed
  installs and unknown prior state are never wiped.
- Package npm specs pin the package version but not the whole transitive npm
  tree. This backend does not promise whole-machine rollback or sandboxing.

## Wizard integration (G4/G5) and out of scope

The retained wizard now calls `Prepare`, renders `Plan.Steps` (outputs and
effects), captures informed approval bound to `Plan.Fingerprint()`, calls
`Execute`, and uses `Report.Launch` for later launches. `--plan-only` previews
without writing; `--non-interactive` alone never authorizes installation without
a matching `--approve-plan <fingerprint>`; `--metadata-only` generates local
artifacts and attempts the Engram write-through. The wizard owns the interactive
UI, the approval and memory-outcome reporting, and workspace generation.

This is source and local-fixture parity only. Full native review and live
installer/loaded-Pi consumer acceptance remain pending, and the pinned candidates
are not certified composition.

Out of scope here: running real installers or downloads on the developer host,
reading `~/.pi` auth/model/settings/profile files, running Godot gameplay, and SDD.

## Verification

```sh
gofmt -w internal/piinstall/*.go
GOTOOLCHAIN=go1.26.2 GOTELEMETRY=off GOPROXY=off GOSUMDB=sum.golang.org go test ./internal/piinstall ./skills -count=1 -timeout 90s
```

Tests use `t.TempDir`, fake command runners and downloaders, generated local
tar.gz/zip fixtures, and a loopback TLS server. No public download, npm/Pi/
Shell/Engram/Godot execution, or private configuration read occurs.
