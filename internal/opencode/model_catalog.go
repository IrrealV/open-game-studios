package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func LoadModelCatalog(providers []ConnectedProvider) (map[string][]string, error) {
	modelPath := resolveModelPath()
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fallbackCatalog(providers), nil
		}
		return nil, fmt.Errorf("read opencode model catalog: %w", err)
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse opencode model catalog: %w", err)
	}

	catalog := map[string][]string{}
	extractCatalog(payload, catalog)

	for provider, models := range catalog {
		catalog[provider] = uniqueSorted(models)
	}

	for _, provider := range providers {
		name := normalizeProvider(provider.Name)
		if len(catalog[name]) == 0 {
			catalog[name] = []string{"auto"}
		}
	}

	return catalog, nil
}

func resolveModelPath() string {
	if custom := strings.TrimSpace(os.Getenv("OPENCODE_MODELS_PATH")); custom != "" {
		return custom
	}

	if dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dataHome != "" {
		return filepath.Join(dataHome, "opencode", "models.json")
	}

	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return filepath.Join(".local", "share", "opencode", "models.json")
	}

	return filepath.Join(home, ".local", "share", "opencode", "models.json")
}

func extractCatalog(payload any, catalog map[string][]string) {
	switch value := payload.(type) {
	case map[string]any:
		for key, child := range value {
			normalized := normalizeProvider(key)
			if models := parseModelList(child); len(models) > 0 {
				catalog[normalized] = append(catalog[normalized], models...)
				continue
			}
			if normalized == "providers" {
				extractCatalog(child, catalog)
			}
		}
	case []any:
		for _, item := range value {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := providerName(entry)
			if name == "" {
				continue
			}
			if models := parseModelList(entry["models"]); len(models) > 0 {
				catalog[name] = append(catalog[name], models...)
			}
		}
	}
}

func parseModelList(value any) []string {
	if value == nil {
		return nil
	}

	switch items := value.(type) {
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			switch v := item.(type) {
			case string:
				if model := strings.TrimSpace(v); model != "" {
					out = append(out, model)
				}
			case map[string]any:
				if raw, ok := v["id"].(string); ok {
					if model := strings.TrimSpace(raw); model != "" {
						out = append(out, model)
					}
				}
			}
		}
		return out
	case map[string]any:
		if nested, ok := items["models"]; ok {
			return parseModelList(nested)
		}
		if raw, ok := items["id"].(string); ok {
			if model := strings.TrimSpace(raw); model != "" {
				return []string{model}
			}
		}
	}

	return nil
}

func fallbackCatalog(providers []ConnectedProvider) map[string][]string {
	catalog := map[string][]string{}
	for _, provider := range providers {
		catalog[normalizeProvider(provider.Name)] = []string{"auto"}
	}
	return catalog
}

func uniqueSorted(items []string) []string {
	seen := map[string]struct{}{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for item := range seen {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
