package routing

import (
	"strings"
	"testing"

	"open-game-studios/internal/opencode"
)

func TestNewBalancedPolicy_ErrorsWhenSnapshotHasNoModels(t *testing.T) {
	snapshot := opencode.ModelSnapshot{}

	_, err := NewBalancedPolicy(snapshot)
	if err == nil {
		t.Fatalf("expected error for empty snapshot")
	}
}

func TestNewBalancedPolicy_BuildsDefaultsFromHints(t *testing.T) {
	snapshot := opencode.ModelSnapshot{
		Providers: []opencode.ProviderSnapshot{
			{Provider: "openai", Models: []string{"gpt-4o", "gpt-4o-mini", "gpt-5-pro", "deep-thinker"}},
			{Provider: "azure", Models: []string{"azure-4o", "azure-3-mini"}},
		},
	}

	policy, err := NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("NewBalancedPolicy() failed: %v", err)
	}

	if policy.Preset != PresetBalanced {
		t.Fatalf("expected preset %q, got %q", PresetBalanced, policy.Preset)
	}

	if got := policy.TierDefaults[TierFast]; got != (Selection{Provider: "openai", Model: "gpt-4o-mini"}) {
		t.Fatalf("unexpected fast tier default: %#v", got)
	}
	if got := policy.TierDefaults[TierBalanced]; got != (Selection{Provider: "openai", Model: "gpt-4o"}) {
		t.Fatalf("unexpected balanced tier default: %#v", got)
	}
	if got := policy.TierDefaults[TierDeep]; got != (Selection{Provider: "openai", Model: "gpt-5-pro"}) {
		t.Fatalf("unexpected deep tier default: %#v", got)
	}
}

func TestPolicyOverrides_ApplyRoleOverride(t *testing.T) {
	snapshot := opencode.ModelSnapshot{
		Providers: []opencode.ProviderSnapshot{{Provider: "openai", Models: []string{"gpt-4o", "gpt-4o-mini"}}},
	}
	policy, err := NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("NewBalancedPolicy() failed: %v", err)
	}

	if err := policy.ApplyRoleOverride(snapshot, "ORCHESTRATOR", Selection{Provider: "OPENAI", Model: "gpt-4o"}); err != nil {
		t.Fatalf("ApplyRoleOverride() failed: %v", err)
	}

	override := policy.RoleOverrides["orchestrator"]
	if override.Provider != "openai" {
		t.Fatalf("expected normalized provider override, got %q", override.Provider)
	}
	if override.Model != "gpt-4o" {
		t.Fatalf("expected model override gpt-4o, got %q", override.Model)
	}
}

func TestPolicyOverrides_RejectInvalidOverrides(t *testing.T) {
	snapshot := opencode.ModelSnapshot{
		Providers: []opencode.ProviderSnapshot{{Provider: "openai", Models: []string{"gpt-4o-mini"}}},
	}
	policy, err := NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("NewBalancedPolicy() failed: %v", err)
	}

	tests := []struct {
		name string
		fn   func() error
		want string
	}{
		{name: "empty role", fn: func() error {
			return policy.ApplyRoleOverride(snapshot, " ", Selection{Provider: "openai", Model: "gpt-4o"})
		}, want: "role cannot be empty"},
		{name: "missing model", fn: func() error {
			return policy.ApplyRoleOverride(snapshot, "orchestrator", Selection{Provider: "openai", Model: ""})
		}, want: "provider/model selection cannot be empty"},
		{name: "unknown model", fn: func() error {
			return policy.ApplyPhaseOverride(snapshot, "explore", Selection{Provider: "openai", Model: "nope"})
		}, want: "not present in connected-provider snapshot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatalf("expected error for %q", tt.name)
			}
			if err.Error() == "" {
				t.Fatalf("expected non-empty error")
			}
			if tt.want != "" && !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
