package routing

import (
	"fmt"
	"strings"

	"open-game-studios/internal/opencode"
)

type Tier string

const (
	TierFast     Tier = "fast"
	TierBalanced Tier = "balanced"
	TierDeep     Tier = "deep"
)

const PresetBalanced = "balanced"

var DefaultRoles = []string{"orchestrator", "implementer", "reviewer"}
var DefaultPhases = []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"}

type Selection struct {
	Provider string
	Model    string
}

type Policy struct {
	Preset         string
	TierDefaults   map[Tier]Selection
	RoleOverrides  map[string]Selection
	PhaseOverrides map[string]Selection
}

func NewBalancedPolicy(snapshot opencode.ModelSnapshot) (Policy, error) {
	all := availableSelections(snapshot)
	if len(all) == 0 {
		return Policy{}, fmt.Errorf("model snapshot has no selectable models")
	}

	base := all[0]

	policy := Policy{
		Preset: PresetBalanced,
		TierDefaults: map[Tier]Selection{
			TierFast:     chooseByHint(all, base, []string{"mini", "flash", "haiku", "lite"}),
			TierBalanced: chooseByHint(all, base, []string{"sonnet", "4o", "balanced", "medium"}),
			TierDeep:     chooseByHint(all, base, []string{"opus", "pro", "o1", "deep"}),
		},
		RoleOverrides:  map[string]Selection{},
		PhaseOverrides: map[string]Selection{},
	}

	return policy, nil
}

func (p *Policy) ApplyRoleOverride(snapshot opencode.ModelSnapshot, role string, selection Selection) error {
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	if normalizedRole == "" {
		return fmt.Errorf("role cannot be empty")
	}
	if err := validateSelection(snapshot, selection); err != nil {
		return err
	}

	p.RoleOverrides[normalizedRole] = normalizeSelection(selection)
	return nil
}

func (p *Policy) ApplyPhaseOverride(snapshot opencode.ModelSnapshot, phase string, selection Selection) error {
	normalizedPhase := strings.ToLower(strings.TrimSpace(phase))
	if normalizedPhase == "" {
		return fmt.Errorf("phase cannot be empty")
	}
	if err := validateSelection(snapshot, selection); err != nil {
		return err
	}

	p.PhaseOverrides[normalizedPhase] = normalizeSelection(selection)
	return nil
}

func (p Policy) ComplexitySummary(complexity string) string {
	switch strings.ToLower(strings.TrimSpace(complexity)) {
	case "simple":
		return "Simple: preset balanced por tier (sin overrides manuales)"
	case "advanced":
		return "Advanced: preset balanced + overrides por rol"
	case "expert":
		return "Expert: preset balanced + overrides por rol y por fase"
	default:
		return "Complexity no reconocida"
	}
}

func validateSelection(snapshot opencode.ModelSnapshot, selection Selection) error {
	provider := strings.ToLower(strings.TrimSpace(selection.Provider))
	model := strings.TrimSpace(selection.Model)
	if provider == "" || model == "" {
		return fmt.Errorf("provider/model selection cannot be empty")
	}
	if !snapshot.HasModel(provider, model) {
		return fmt.Errorf("invalid selection %s:%s (not present in connected-provider snapshot)", provider, model)
	}
	return nil
}

func normalizeSelection(selection Selection) Selection {
	return Selection{
		Provider: strings.ToLower(strings.TrimSpace(selection.Provider)),
		Model:    strings.TrimSpace(selection.Model),
	}
}

func availableSelections(snapshot opencode.ModelSnapshot) []Selection {
	items := make([]Selection, 0)
	for _, provider := range snapshot.Providers {
		for _, model := range provider.Models {
			items = append(items, Selection{Provider: provider.Provider, Model: model})
		}
	}
	return items
}

func chooseByHint(all []Selection, fallback Selection, hints []string) Selection {
	for _, item := range all {
		name := strings.ToLower(item.Model)
		for _, hint := range hints {
			if strings.Contains(name, hint) {
				return item
			}
		}
	}
	return fallback
}
