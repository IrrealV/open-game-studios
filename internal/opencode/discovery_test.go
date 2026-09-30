package opencode

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListConnectedProviders(t *testing.T) {
	tests := []struct {
		name        string
		authJSON    string
		want        []ConnectedProvider
		wantAuthErr bool
		wantNoneErr bool
		missingFile bool
	}{
		{
			name: "returns only connected providers normalized and sorted",
			authJSON: `{
			  "providers": {
			    "OpenAI": {"connected": true},
			    "anthropic": {"connected": false}
			  },
			  "connections": [
			    {"provider": "Groq", "connected": true}
			  ]
			}`,
			want: []ConnectedProvider{{Name: "groq"}, {Name: "openai"}},
		},
		{
			name: "returns no-connected-providers error when auth exists but disconnected",
			authJSON: `{
			  "providers": {
			    "OpenAI": {"connected": false},
			    "anthropic": {"status": "disconnected"}
			  }
			}`,
			wantNoneErr: true,
		},
		{
			name: "does not treat disconnected provider with token as connected",
			authJSON: `{
			  "providers": {
			    "OpenAI": {"connected": false, "token": "abc123"},
			    "Anthropic": {"status": "disconnected", "token": "xyz"}
			  }
			}`,
			wantNoneErr: true,
		},
		{
			name:        "returns auth-missing error when file does not exist",
			missingFile: true,
			wantAuthErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			authPath := filepath.Join(tempDir, "auth.json")
			if !tt.missingFile {
				if err := os.WriteFile(authPath, []byte(tt.authJSON), 0o644); err != nil {
					t.Fatalf("write auth fixture: %v", err)
				}
			}

			t.Setenv("OPENCODE_AUTH_PATH", authPath)

			got, err := ListConnectedProviders()

			if tt.wantAuthErr {
				if !IsAuthMissing(err) {
					t.Fatalf("expected auth missing error, got: %v", err)
				}
				return
			}

			if tt.wantNoneErr {
				if !IsNoConnectedProviders(err) {
					t.Fatalf("expected no connected providers error, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ListConnectedProviders returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("providers mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}
