package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ProviderHelperText = "Solo se muestran los providers conectados a opencode"

var ErrAuthFileNotFound = errors.New("opencode auth file not found")
var ErrNoConnectedProviders = errors.New("no connected providers")

type ConnectedProvider struct {
	Name string
}

func ListConnectedProviders() ([]ConnectedProvider, error) {
	authPath := resolveAuthPath()
	raw, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w at %s", ErrAuthFileNotFound, authPath)
		}
		return nil, fmt.Errorf("read opencode auth file: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse opencode auth file: %w", err)
	}

	names := extractConnectedProviderNames(payload)
	if len(names) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoConnectedProviders, authPath)
	}

	providers := make([]ConnectedProvider, 0, len(names))
	for _, name := range names {
		providers = append(providers, ConnectedProvider{Name: name})
	}

	return providers, nil
}

func IsAuthMissing(err error) bool {
	return errors.Is(err, ErrAuthFileNotFound)
}

func IsNoConnectedProviders(err error) bool {
	return errors.Is(err, ErrNoConnectedProviders)
}

func resolveAuthPath() string {
	if custom := strings.TrimSpace(os.Getenv("OPENCODE_AUTH_PATH")); custom != "" {
		return custom
	}

	if dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dataHome != "" {
		return filepath.Join(dataHome, "opencode", "auth.json")
	}

	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return filepath.Join(".local", "share", "opencode", "auth.json")
	}

	return filepath.Join(home, ".local", "share", "opencode", "auth.json")
}

func extractConnectedProviderNames(payload map[string]any) []string {
	seen := map[string]struct{}{}

	collectProviderMap := func(value any) {
		providers, ok := value.(map[string]any)
		if !ok {
			return
		}
		for provider, config := range providers {
			if isConnected(config) {
				seen[normalizeProvider(provider)] = struct{}{}
			}
		}
	}

	collectProviderList := func(value any) {
		items, ok := value.([]any)
		if !ok {
			return
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok || !isConnected(entry) {
				continue
			}
			if name := providerName(entry); name != "" {
				seen[name] = struct{}{}
			}
		}
	}

	collectProviderMap(payload["providers"])
	collectProviderMap(payload["connections"])
	collectProviderMap(payload["auth"])

	collectProviderList(payload["providers"])
	collectProviderList(payload["connections"])

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func providerName(entry map[string]any) string {
	for _, key := range []string{"provider", "name", "id"} {
		if raw, ok := entry[key].(string); ok {
			if normalized := normalizeProvider(raw); normalized != "" {
				return normalized
			}
		}
	}
	return ""
}

func normalizeProvider(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isConnected(value any) bool {
	if flag, ok := value.(bool); ok {
		return flag
	}

	entry, ok := value.(map[string]any)
	if !ok {
		return false
	}

	if connected, ok := entry["connected"].(bool); ok {
		return connected
	}

	if status, ok := entry["status"].(string); ok {
		normalized := strings.ToLower(strings.TrimSpace(status))
		switch normalized {
		case "disconnected", "disabled", "unauthenticated", "revoked", "invalid":
			return false
		case "connected", "authenticated", "ok":
			return true
		}
	}

	for _, key := range []string{"token", "api_key", "access_token", "refresh_token"} {
		if token, ok := entry[key].(string); ok && strings.TrimSpace(token) != "" {
			return true
		}
	}

	return false
}
