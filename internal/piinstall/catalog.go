package piinstall

import ogsskills "open-game-studios/skills"

// Component identifies a prerequisite or payload managed by the Pi installer.
type Component string

const (
	ComponentNode            Component = "node"
	ComponentNpm             Component = "npm"
	ComponentPi              Component = "pi"
	ComponentShell           Component = "gentle-shell"
	ComponentEngramCore      Component = "engram-core"
	ComponentEngramCompanion Component = "gentle-engram"
	ComponentGodot           Component = "godot"
	ComponentOGSPayload      Component = "ogs-payload"
)

// Ordinary Pi package sources pinned by the published candidate catalog. These
// are installed with the public `pi install <source> --no-approve` command and
// are never installed as standalone launchers or via `gentle-ai install`.
const (
	PiNpmPackage                 = "@earendil-works/pi-coding-agent@0.87.1"
	ShellNpmPackage              = "gentle-pi@3.7.0"
	EngramCompanionNpmPackage    = "gentle-engram@0.1.15"
	PiPackageSource              = "npm:" + PiNpmPackage
	ShellPackageSource           = "npm:" + ShellNpmPackage
	EngramCompanionPackageSource = "npm:" + EngramCompanionNpmPackage
)

// Candidate artifact kinds.
const (
	KindNpm      = "npm"
	KindArchive  = "archive"
	KindEmbedded = "embedded"
)

// Candidate pins one published artifact from the OGS task document. These are
// candidates, not certified integration evidence: archive contents, transitive
// npm lifecycle effects and runtime interoperability remain unverified until a
// separate consented consumer acceptance runs.
//
// Size is 0 when the task document did not pin an exact byte count. Node's
// gzip digest is pinned, but its archive size is intentionally left unverified.
type Candidate struct {
	Component Component
	Version   string
	Kind      string
	Source    string
	SHA256    string
	Size      int64
	Note      string
}

var candidates = []Candidate{
	{
		Component: ComponentNode,
		Version:   "24.21.0",
		Kind:      KindArchive,
		Source:    "https://nodejs.org/dist/v24.21.0/node-v24.21.0-linux-x64.tar.gz",
		SHA256:    "6e1db87ef58b8819e5d5402eff1536491b18edd8eb7bee5ef7897876e88dc5ff",
		Note:      "Official Linux x64 tar.gz; archive size not pinned. tar contains legitimate in-root npm/npx links.",
	},
	{
		Component: ComponentPi,
		Version:   "0.87.1",
		Kind:      KindNpm,
		Source:    PiNpmPackage,
		Note:      "Installed with npm --global --prefix <owned prefix> --ignore-scripts when absent. Requires Node >= 22.19.0.",
	},
	{
		Component: ComponentShell,
		Version:   "3.7.0",
		Kind:      KindNpm,
		Source:    ShellPackageSource,
		Note:      "Ordinary Pi package; postinstall manages Shell's package-private native runtime and can persist personal tuiMode fullscreen.",
	},
	{
		Component: ComponentEngramCore,
		Version:   "2.2.0",
		Kind:      KindArchive,
		Source:    "https://github.com/Gentleman-Programming/engram/releases/download/v2.2.0/engram_2.2.0_linux_amd64.tar.gz",
		SHA256:    "20cacc7ee62e4bc21df20deb68e75dc8fd5db529d5957b19644aa7781df486bd",
		Size:      7600693,
		Note:      "Core binary only; no exact 2.2.0 companion compatibility certification is claimed.",
	},
	{
		Component: ComponentEngramCompanion,
		Version:   "0.1.15",
		Kind:      KindNpm,
		Source:    EngramCompanionPackageSource,
		Note:      "Pi-native companion; requires the Engram core binary through PATH or ENGRAM_BIN. Not MCP or model setup.",
	},
	{
		Component: ComponentGodot,
		Version:   "4.7.2",
		Kind:      KindArchive,
		Source:    "https://github.com/godotengine/godot-builds/releases/download/4.7.2-stable/Godot_v4.7.2-stable_linux.x86_64.zip",
		SHA256:    "cadd3204e728a35d3f13adb7fd0d7902636b79f6b95c40c265eb73b6c35329e4",
		Size:      77860424,
		Note:      "Optional; installed or reused only when the caller marks Godot as required. Gameplay is never run.",
	},
	{
		Component: ComponentOGSPayload,
		Version:   "1.0",
		Kind:      KindEmbedded,
		Source:    "embedded:" + ogsskills.SkillsRoot,
		Note:      "Canonical OGS skills (ogs-godot-change and ogs-core) compiled into the installer binary and materialized under the hardwired .pi/skills root.",
	},
}

// Candidates returns a copy of the pinned candidate catalog.
func Candidates() []Candidate {
	return append([]Candidate(nil), candidates...)
}

// CandidateFor returns the pinned candidate for a component.
func CandidateFor(component Component) (Candidate, bool) {
	for _, candidate := range candidates {
		if candidate.Component == component {
			return candidate, true
		}
	}
	return Candidate{}, false
}
