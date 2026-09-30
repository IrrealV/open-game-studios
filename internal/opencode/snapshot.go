package opencode

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ProviderSnapshot struct {
	Provider string
	Models   []string
}

type ModelSnapshot struct {
	Profile    string
	CapturedAt time.Time
	Providers  []ProviderSnapshot
}

func CaptureModelSnapshot(profile string, providers []ConnectedProvider) (ModelSnapshot, error) {
	if strings.TrimSpace(profile) == "" {
		return ModelSnapshot{}, fmt.Errorf("profile name is required")
	}
	if len(providers) == 0 {
		return ModelSnapshot{}, fmt.Errorf("cannot capture snapshot without connected providers")
	}

	catalog, err := LoadModelCatalog(providers)
	if err != nil {
		return ModelSnapshot{}, err
	}

	snapshotProviders := make([]ProviderSnapshot, 0, len(providers))
	for _, provider := range providers {
		name := normalizeProvider(provider.Name)
		models := catalog[name]
		if len(models) == 0 {
			models = []string{"auto"}
		}
		snapshotProviders = append(snapshotProviders, ProviderSnapshot{
			Provider: name,
			Models:   uniqueSorted(models),
		})
	}

	sort.Slice(snapshotProviders, func(i, j int) bool {
		return snapshotProviders[i].Provider < snapshotProviders[j].Provider
	})

	return ModelSnapshot{
		Profile:    strings.TrimSpace(profile),
		CapturedAt: time.Now().UTC(),
		Providers:  snapshotProviders,
	}, nil
}

func (m ModelSnapshot) HasModel(provider, model string) bool {
	p := normalizeProvider(provider)
	target := strings.TrimSpace(model)
	if p == "" || target == "" {
		return false
	}

	for _, providerSnapshot := range m.Providers {
		if providerSnapshot.Provider != p {
			continue
		}
		for _, candidate := range providerSnapshot.Models {
			if candidate == target {
				return true
			}
		}
	}

	return false
}
