package persistence

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Placeholder struct {
	hybrid     HybridWiring
	engramExec string
}

// WithEngramExecutable returns a copy that runs Engram saves through the given
// executable instead of resolving `engram` on PATH. The wizard supplies the
// backend-reported Launch.EngramBinary here so a managed install is used without
// editing ambient PATH. An empty path keeps the historical engram-on-PATH
// behavior for every default caller.
func (p Placeholder) WithEngramExecutable(executable string) Placeholder {
	p.engramExec = strings.TrimSpace(executable)
	return p
}

// WithEngramEnabled returns a copy with the Engram write-through enabled or
// disabled. A disabled placeholder performs no memory save and reports the
// outcome as explicitly skipped rather than saved.
func (p Placeholder) WithEngramEnabled(enabled bool) Placeholder {
	p.hybrid.Engram.Enabled = enabled
	return p
}

type HybridWiring struct {
	Engram   Endpoint
	OpenSpec Endpoint
	Context7 Endpoint
}

type Endpoint struct {
	Enabled bool
	Name    string
}

func NewPlaceholder() Placeholder {
	return Placeholder{
		hybrid: HybridWiring{
			// Engram (Pi-native companion) and OpenSpec are the real hybrid
			// memory/spec backends. Context7 and other MCP adapters are optional and
			// are not implicitly enabled.
			Engram:   Endpoint{Enabled: true, Name: "engram"},
			OpenSpec: Endpoint{Enabled: true, Name: "openspec"},
			Context7: Endpoint{Enabled: false, Name: "context7"},
		},
	}
}

func (p Placeholder) Mode() string {
	return "hybrid"
}

func (p Placeholder) Wiring() HybridWiring {
	return p.hybrid
}

type GenerationWriteThrough struct {
	ProfileName      string
	Engine           string
	PersistenceMode  string
	SummaryArtifact  string
	GeneratedAt      time.Time
	EmittedArtifacts []string
}

func (p Placeholder) WriteGeneration(input GenerationWriteThrough) error {
	return p.WriteGenerationContext(context.Background(), input)
}

// WriteGenerationContext performs the same local artifact and Engram
// write-through as WriteGeneration while honoring the caller's context. The
// Engram save keeps its bounded timeout, derived from the caller context so a
// canceled wizard run cannot proceed to the memory write.
func (p Placeholder) WriteGenerationContext(ctx context.Context, input GenerationWriteThrough) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !p.hybrid.Engram.Enabled {
		return nil
	}

	summaryArtifact := sanitizeArtifactPath(input.SummaryArtifact)
	emittedArtifacts := make([]string, 0, len(input.EmittedArtifacts))
	for _, artifact := range input.EmittedArtifacts {
		emittedArtifacts = append(emittedArtifacts, sanitizeArtifactPath(artifact))
	}

	title := fmt.Sprintf("installer-wizard/final-artifact/%s", strings.TrimSpace(strings.ToLower(input.Engine)))
	body := fmt.Sprintf("## Installer Wizard Final Artifact\n\n- profile: %s\n- engine: %s\n- persistence: %s\n- generated_at: %s\n- summary_artifact: %s\n\n## Emitted Artifacts\n%s", input.ProfileName, input.Engine, input.PersistenceMode, input.GeneratedAt.UTC().Format(time.RFC3339), summaryArtifact, formatArtifactList(emittedArtifacts))

	return saveEngramCLIContext(ctx, SaveInput{
		Title:   title,
		Body:    body,
		Type:    "architecture",
		Scope:   "project",
		Project: "open-game-studios",
	}, p.engramExec)
}

func formatArtifactList(items []string) string {
	if len(items) == 0 {
		return ""
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s", item))
	}

	return strings.Join(lines, "\n")
}

func sanitizeArtifactPath(raw string) string {
	clean := filepath.Clean(strings.TrimSpace(raw))
	if clean == "." || clean == "" {
		return ""
	}

	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean)
	}

	forward := filepath.ToSlash(clean)
	segments := strings.Split(forward, "/")
	// `.opencode` is retained as a legacy anchor so historical serialized paths
	// still sanitize; `.game-studio` is the current first-class workspace anchor.
	for _, anchor := range []string{".opencode", "out", ".game-studio"} {
		for i, segment := range segments {
			if segment == anchor {
				return strings.Join(segments[i:], "/")
			}
		}
	}

	return filepath.Base(forward)
}
