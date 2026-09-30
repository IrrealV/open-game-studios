package commands

import (
	"sort"
	"unicode"

	"open-game-studios/internal/integrations"
	"open-game-studios/internal/toolcheck"
)

func requiredToolIDsFromResources(registry *toolcheck.Registry, resources ...string) []string {
	if registry == nil {
		registry = toolcheck.NewDefaultRegistry()
	}

	seen := map[string]struct{}{}
	for _, resource := range resources {
		resourceParts := resourceTokenParts(resource)
		if len(resourceParts) == 0 {
			continue
		}

		for _, meta := range registry.List() {
			if resourceMatchesToolMetadata(resourceParts, meta) {
				seen[meta.ID] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func optionalDiagnosticToolIDs(registry *toolcheck.Registry, selections []integrations.OptionalIntegrationSelection) []string {
	if registry == nil {
		registry = toolcheck.NewDefaultRegistry()
	}
	integrationRegistry := integrations.NewDefaultRegistry()
	seen := map[string]struct{}{}
	for _, selection := range selections {
		integration, ok := integrationRegistry.Get(selection.ID)
		if !ok {
			continue
		}
		for _, rawID := range integration.DiagnosticToolIDs {
			id := registry.CanonicalID(rawID)
			if id == "" {
				continue
			}
			if _, ok := registry.Get(id); !ok {
				continue
			}
			seen[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func optionalWarningResults(results []toolcheck.Result) []toolcheck.Result {
	out := make([]toolcheck.Result, 0, len(results))
	for _, result := range results {
		result.Severity = toolcheck.SeverityOptional
		result.Required = false
		if result.Status == toolcheck.StatusFailure {
			result.Status = toolcheck.StatusWarning
		}
		out = append(out, result)
	}
	return out
}

func resourceMatchesToolMetadata(resourceParts []string, meta toolcheck.Metadata) bool {
	ids := append([]string{meta.ID}, meta.Aliases...)
	for _, id := range ids {
		idParts := resourceTokenParts(id)
		if len(idParts) == 0 {
			continue
		}
		if containsContiguousParts(resourceParts, idParts) {
			return true
		}
	}
	return false
}

func resourceTokenParts(value string) []string {
	parts := make([]string, 0)
	current := make([]rune, 0, len(value))
	flush := func() {
		if len(current) == 0 {
			return
		}
		parts = append(parts, string(current))
		current = current[:0]
	}

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current = append(current, unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return parts
}

func containsContiguousParts(resourceParts, idParts []string) bool {
	if len(idParts) == 0 || len(idParts) > len(resourceParts) {
		return false
	}
	for start := 0; start <= len(resourceParts)-len(idParts); start++ {
		matched := true
		for offset, idPart := range idParts {
			if resourceParts[start+offset] != idPart {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
