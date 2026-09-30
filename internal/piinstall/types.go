package piinstall

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Sentinel errors returned by the public API. Per-step failures are reported in
// Report.Entries and stop execution; they do not roll back completed steps.
var (
	// ErrConsentRequired means Execute was called without explicit approval.
	ErrConsentRequired = errors.New("piinstall: explicit plan approval is required")
	// ErrPlanDrift means the approved plan fingerprint no longer matches, or an
	// approved plan marks a component as blocked.
	ErrPlanDrift = errors.New("piinstall: approved plan fingerprint does not match the submitted plan")
	// ErrPlanBlocked means the plan contains a fail-closed blocked component.
	ErrPlanBlocked = errors.New("piinstall: plan contains a blocked component")
	// ErrInvalidConfig means required configuration is missing or relative.
	ErrInvalidConfig = errors.New("piinstall: invalid configuration")
	// ErrUnsafeRoot means InstallRoot is not an owned per-user destination: it is
	// a shared/system prefix, is not owned by the current user, is
	// group/world-writable, or is reached through a symlink or non-directory.
	ErrUnsafeRoot = errors.New("piinstall: install root is not an owned per-user destination")
)

// CommandSpec describes one child process invocation. Env, when non-nil,
// replaces the child environment entirely; callers must include the PATH and
// tool directories they intend the child to see.
type CommandSpec struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

// CommandResult is the bounded result of one child process invocation.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// CommandRunner is the injected seam for lookups and command execution. Tests
// replace it with a fake; production uses ExecRunner.
type CommandRunner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, spec CommandSpec) CommandResult
}

// Downloader is the injected seam for bounded archive downloads. Production
// uses HTTPDownloader, which requires an https URL and enforces maxBytes.
type Downloader interface {
	Download(ctx context.Context, rawURL, destPath string, maxBytes int64) error
}

// Config is the caller-provided installer configuration. Construction and the
// read-only Detect/BuildPlan path never write to the host.
type Config struct {
	// WorkspaceDir receives .pi/skills payload output. Required and absolute.
	WorkspaceDir string
	// InstallRoot is the owned per-user root for all managed installs. Required
	// and absolute. It must not be a system or shared prefix.
	InstallRoot string

	// GodotRequired makes Godot a planned prerequisite. Godot is never run
	// beyond a read-only readiness probe.
	GodotRequired bool

	// Optional explicit executable paths. Empty means look up the conventional
	// name on PATH.
	NodePath   string
	NpmPath    string
	PiPath     string
	EngramPath string
	GodotPath  string

	// Injected seams. Nil uses the production implementations.
	Commands   CommandRunner
	Downloader Downloader
	// HTTPClient lets tests supply a TLS client trusting a loopback server.
	HTTPClient *http.Client

	// Timeouts bound command execution and downloads.
	CommandTimeout  time.Duration
	DownloadTimeout time.Duration

	// archiveDigests overrides the expected archive digest and size. It is
	// unexported so production callers cannot weaken pinned verification; only
	// in-package tests substitute local archive fixtures through it.
	archiveDigests map[Component]archiveDigestOverride
}

type archiveDigestOverride struct {
	sha  string
	size int64
}

// normalizeConfigPath trims surrounding whitespace and returns a lexical
// (non-resolving) clean path. It preserves the empty string so an unset
// optional path stays unset instead of collapsing to the current directory.
// Every execution-relevant path is normalized through this single helper so the
// approved binding, validation, and all filesystem/command IO agree on the same
// string.
func normalizeConfigPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func (c Config) withDefaults() Config {
	if c.Commands == nil {
		c.Commands = ExecRunner{Timeout: c.CommandTimeout}
	}
	if c.Downloader == nil {
		c.Downloader = HTTPDownloader{Client: c.HTTPClient, Timeout: c.DownloadTimeout}
	}
	// Normalize the execution-relevant paths exactly once. Detect, BuildPlan,
	// and Execute all call withDefaults, so the validated/approved lexical root
	// and the root used by every managed IO or command are identical. A raw root
	// such as an alias/".." path could otherwise validate against one physical
	// parent while raw IO staged under another.
	c.WorkspaceDir = normalizeConfigPath(c.WorkspaceDir)
	c.InstallRoot = normalizeConfigPath(c.InstallRoot)
	c.NodePath = normalizeConfigPath(c.NodePath)
	c.NpmPath = normalizeConfigPath(c.NpmPath)
	c.PiPath = normalizeConfigPath(c.PiPath)
	c.EngramPath = normalizeConfigPath(c.EngramPath)
	c.GodotPath = normalizeConfigPath(c.GodotPath)
	return c
}

func (c Config) commands() CommandRunner {
	if c.Commands != nil {
		return c.Commands
	}
	return ExecRunner{Timeout: c.CommandTimeout}
}

func (c Config) downloader() Downloader {
	if c.Downloader != nil {
		return c.Downloader
	}
	return HTTPDownloader{Client: c.HTTPClient, Timeout: c.DownloadTimeout}
}

func (c Config) downloadTimeout() time.Duration {
	if c.DownloadTimeout > 0 {
		return c.DownloadTimeout
	}
	return 10 * time.Minute
}

func (c Config) commandTimeout() time.Duration {
	if c.CommandTimeout > 0 {
		return c.CommandTimeout
	}
	return 2 * time.Minute
}

// validate rejects missing, relative, or obviously unsafe roots. InstallRoot is
// the owned per-user destination for every managed install; it must not be a
// shared or system prefix.
func (c Config) validate() error {
	if c.WorkspaceDir == "" || c.InstallRoot == "" {
		return ErrInvalidConfig
	}
	if !isAbsPath(c.WorkspaceDir) || !isAbsPath(c.InstallRoot) {
		return ErrInvalidConfig
	}
	if isSharedSystemPath(c.InstallRoot) {
		return fmt.Errorf("%w: %s is a shared or system prefix; choose an owned per-user root such as one under the user's home or an OS temp directory", ErrUnsafeRoot, c.InstallRoot)
	}
	if err := validateOwnedRoot(c.InstallRoot); err != nil {
		return err
	}
	return nil
}

// systemPrefixes are shared system trees that must never receive a managed
// install. Paths under these prefixes are rejected.
var systemPrefixes = []string{
	"/usr", "/etc", "/bin", "/sbin", "/lib", "/lib64", "/boot",
	"/sys", "/proc", "/dev", "/opt", "/root", "/var",
}

// sharedRoots are shared top-level roots that are only rejected exactly; the
// per-user directories beneath them remain valid owned destinations.
var sharedRoots = []string{"/home", "/Users", "/Volumes"}

// tempRoots are the shared temporary roots that are never themselves owned
// per-user roots, although a private per-user directory beneath them is valid.
// They are a fixed platform list rather than os.TempDir() so that a
// caller-configured TMPDIR (for example /usr) can never bypass the system
// prefix protection below.
var tempRoots = func() []string {
	roots := []string{"/tmp", "/var/tmp"}
	if runtime.GOOS == "darwin" {
		roots = append(roots, "/private/tmp", "/private/var/tmp", "/var/folders", "/private/var/folders")
	}
	return roots
}()

var windowsSystemPrefixes = []string{`c:\windows`, `c:\program files`, `c:\program files (x86)`, `c:\programdata`}

// isSharedSystemPath reports whether candidate is a shared/system prefix rather
// than an owned per-user destination. A private per-user directory beneath a
// known temp root stays valid, but the temp root itself and anything under a
// system prefix is rejected. This is a policy check, not a sandbox: it does not
// scan host configuration and does not resolve symlinks, so callers must still
// treat InstallRoot as trusted input. Real ownership/permission metadata is
// checked separately by validateOwnedRoot.
func isSharedSystemPath(candidate string) bool {
	clean := filepath.Clean(candidate)
	if clean == "" || clean == "." {
		return false
	}
	if clean == string(filepath.Separator) {
		return true
	}
	for _, root := range sharedRoots {
		if clean == root {
			return true
		}
	}
	for _, temp := range tempRoots {
		switch {
		case clean == temp:
			return true
		case sameOrWithin(clean, temp):
			return false
		}
	}
	for _, prefix := range systemPrefixes {
		if sameOrWithin(clean, prefix) {
			return true
		}
	}
	if runtime.GOOS == "windows" {
		lower := strings.ToLower(clean)
		for _, prefix := range windowsSystemPrefixes {
			if lower == prefix || strings.HasPrefix(lower, prefix+`\`) {
				return true
			}
		}
	}
	return false
}

func sameOrWithin(candidate, prefix string) bool {
	if candidate == prefix {
		return true
	}
	return strings.HasPrefix(candidate, prefix+string(filepath.Separator))
}

// Compatibility classifies an observed or embedded component.
type Compatibility string

const (
	// CompatCompatible means the observed executable meets the minimum for the
	// pinned candidate and can be reused.
	CompatCompatible Compatibility = "compatible"
	// CompatIncompatible means a version was read and does not meet the minimum.
	CompatIncompatible Compatibility = "incompatible"
	// CompatUnknown means the probe output could not be classified safely.
	CompatUnknown Compatibility = "unknown"
	// CompatAbsent means the component was not found.
	CompatAbsent Compatibility = "absent"
	// CompatModified means an existing payload file differs from the embedded one.
	CompatModified Compatibility = "modified"
)

// ComponentState is one read-only detection observation.
type ComponentState struct {
	Component     Component
	Found         bool
	Path          string
	Version       string
	Compatibility Compatibility
	// ProbeDeferred marks components whose presence can only be established by
	// a post-consent command (Pi package listings), never during preview.
	ProbeDeferred bool
	Detail        string
}

// Detection is the read-only, zero-write prerequisite snapshot. Package
// presence is always deferred to post-consent probing.
type Detection struct {
	States []ComponentState
}

// State returns the recorded state for a component.
func (d Detection) State(component Component) (ComponentState, bool) {
	for _, state := range d.States {
		if state.Component == component {
			return state, true
		}
	}
	return ComponentState{}, false
}

// Action is a proposed, conditional plan operation.
type Action string

const (
	// ActionReuse keeps an existing compatible component.
	ActionReuse Action = "reuse"
	// ActionInstall creates a missing component at an owned destination.
	ActionInstall Action = "install"
	// ActionEnsure resolves post-consent: reuse an exact pinned registration, or
	// install when genuinely absent. Conflicting or unreadable output fails closed.
	ActionEnsure Action = "ensure"
	// ActionBlocked refuses to proceed without human resolution.
	ActionBlocked Action = "blocked"
)

// PlanStep is one immutable conditional action in the proposal.
type PlanStep struct {
	Component Component
	Action    Action
	Reason    string
	// Outputs lists planned destination paths.
	Outputs []string
	// Effects discloses side effects a human must weigh before consent.
	Effects []string
	// Command is the planned command line, or a short description for archive work.
	Command string
	// Observed is the detection snapshot this step was derived from. Execute
	// re-probes and fails closed when the observed path/version drifts.
	Observed ComponentState
}

// Plan is a read-only proposal. Building or inspecting a Plan performs no
// writes and no mutating commands.
type Plan struct {
	Steps         []PlanStep
	GodotRequired bool
	// binding captures the normalized configuration that determines execution
	// destinations. Fingerprint binds both this and every step so a changed
	// root, workspace, explicit path, output, effect, or command refuses before
	// any write.
	binding planBinding
}

// Fingerprint returns the deterministic digest of the plan's conditional
// actions. Execute requires a matching Consent.PlanFingerprint.
func (p Plan) Fingerprint() string {
	return fingerprintOf(p)
}

// Blocked reports whether any step requires human resolution before execution.
func (p Plan) Blocked() bool {
	for _, step := range p.Steps {
		if step.Action == ActionBlocked {
			return true
		}
	}
	return false
}

// NeedsConsent reports whether the plan would change the host. A pure reuse
// plan still requires explicit approval so that no action is implicit.
func (p Plan) NeedsConsent() bool {
	for _, step := range p.Steps {
		switch step.Action {
		case ActionInstall, ActionEnsure, ActionBlocked:
			return true
		}
	}
	return false
}

// Consent is explicit human approval of a specific plan fingerprint. There is
// no implicit default and a zero-value Consent is never approved.
type Consent struct {
	Approved        bool
	PlanFingerprint string
}

// Outcome is the observed result for one component.
type Outcome string

const (
	OutcomeReused     Outcome = "reused"
	OutcomeInstalled  Outcome = "installed"
	OutcomeDeclined   Outcome = "declined"
	OutcomeFailed     Outcome = "failed"
	OutcomeUnverified Outcome = "unverified"
	OutcomeBlocked    Outcome = "blocked"
)

// Entry is one per-component report row.
type Entry struct {
	Component Component
	Outcome   Outcome
	Detail    string
	Path      string
	Version   string
}

// LaunchInfo is the explicit launch environment the caller should use for
// later real work. It is returned, not applied: the installer never edits the
// user's shell, PATH, or profiles.
type LaunchInfo struct {
	// PiExecutable is the resolved or managed Pi executable.
	PiExecutable string
	// PiPrefix is the owned npm prefix Pi was managed under, when applicable.
	PiPrefix string
	// NodeBinDir is the managed or reused Node bin directory.
	NodeBinDir string
	// NpmExecutable is the npm used for managed installs.
	NpmExecutable string
	// EngramBinary is the resolved or installed Engram core binary.
	EngramBinary string
	// GodotBinary is the resolved or installed Godot executable when required.
	GodotBinary string
	// PathAdditions lists directories a later real launch may prepend to PATH.
	PathAdditions []string
}

// PayloadFileResult reports one embedded payload file.
type PayloadFileResult struct {
	RelativePath string
	Status       string
}

// Payload report statuses.
const (
	PayloadInstalled = "installed"
	PayloadIdentical = "identical"
	PayloadPreserved = "preserved"
)

// PayloadReport reports the workspace payload outcome, including preserved
// modified files that were deliberately not overwritten.
type PayloadReport struct {
	Destination string
	Files       []PayloadFileResult
}

// Report is the observed per-component execution report. Failed steps stop
// further execution; completed steps are never rolled back.
type Report struct {
	Consent Consent
	Entries []Entry
	Launch  LaunchInfo
	Payload PayloadReport
	Failed  bool
	Steps   []PlanStep
	// Notes carries caller-facing follow-up guidance that the installer does not
	// enforce, such as Pi trust/reload instructions. It never claims a live load.
	Notes []string
}
