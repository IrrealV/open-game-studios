package templates

import (
	"strings"
	"testing"
)

// TestProfileTemplatesUsePiOnlyStudioWording guards the G5c parity contract:
// active templates must not advertise an OpenCode profile, credential, or
// provider-discovery surface.
func TestProfileTemplatesUsePiOnlyStudioWording(t *testing.T) {
	registry := NewRegistry()
	for _, id := range []string{
		"game-studio/godot/profile",
		"game-studio/godot/profile-summary",
		"game-studio/unity/profile",
		"game-studio/ue5/profile",
	} {
		t.Run(id, func(t *testing.T) {
			meta, ok := registry.Get(id)
			if !ok {
				t.Fatalf("template %q not registered", id)
			}
			source, err := FS.ReadFile(meta.Path)
			if err != nil {
				t.Fatalf("read template %s: %v", meta.Path, err)
			}
			text := string(source)
			for _, forbidden := range []string{"OpenCode", "auth.json", "provider discovery", "connected providers"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("template %s must not contain %q", meta.Path, forbidden)
				}
			}
		})
	}
}

// TestGodotProfileTemplatesUseCanonicalBindingSource keeps the rendered default
// binding source aligned with the model-routing contract.
func TestGodotProfileTemplatesUseCanonicalBindingSource(t *testing.T) {
	registry := NewRegistry()
	for _, id := range []string{"game-studio/godot/profile", "game-studio/godot/profile-summary"} {
		meta, ok := registry.Get(id)
		if !ok {
			t.Fatalf("template %q not registered", id)
		}
		source, err := FS.ReadFile(meta.Path)
		if err != nil {
			t.Fatalf("read template %s: %v", meta.Path, err)
		}
		if !strings.Contains(string(source), "manual_or_runtime_owned") {
			t.Fatalf("template %s must render the canonical binding source", meta.Path)
		}
	}
}
