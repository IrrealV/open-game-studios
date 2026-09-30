package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpDescribesStagedPiWizard(t *testing.T) {
	help := New().help()
	if !strings.Contains(help, "wizard      Run staged installer/personalization wizard (Pi only)") {
		t.Fatalf("expected help to describe staged Pi-only wizard, got: %s", help)
	}
	if strings.Contains(help, "15-step") {
		t.Fatalf("help must not describe the wizard as a fixed 15-step flow: %s", help)
	}
}

func TestHelpDescribesBriefCommand(t *testing.T) {
	help := New().help()
	if !strings.Contains(help, "brief       Build a pending core-game draft") {
		t.Fatalf("expected help to describe the brief command, got: %s", help)
	}
	if !strings.Contains(help, "brief check is consistency validation") {
		t.Fatalf("expected help to state the brief trust boundary, got: %s", help)
	}
}

func TestAppDispatchRunsBriefDraft(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, "request.json")
	request := `{"Mode":"zero-to-one creation","Request":"Draft a synthetic slice.","Target":"Godot handoff","Contents":[{"Section":"game concept","Body":"c"},{"Section":"core loop","Body":"l"}]}`
	if err := os.WriteFile(requestPath, []byte(request), 0o644); err != nil {
		t.Fatalf("write request fixture: %v", err)
	}

	if err := New().Run([]string{"brief", "draft", "--input", requestPath, "--root", root, "--out", "draft.json"}); err != nil {
		t.Fatalf("Run brief draft: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "draft.json"))
	if err != nil {
		t.Fatalf("read dispatched artifact: %v", err)
	}
	for _, token := range []string{"\"Status\": \"draft\"", "\"ApprovalState\": \"pending_human_approval\"", "\"Revision\""} {
		if !strings.Contains(string(data), token) {
			t.Fatalf("dispatched artifact missing %q, got: %s", token, data)
		}
	}
}

func TestAppDispatchRejectsUnknownBriefSubcommand(t *testing.T) {
	err := New().Run([]string{"brief", "bogus"})
	if err == nil {
		t.Fatalf("expected an unknown brief subcommand to fail")
	}
	if !strings.Contains(err.Error(), "unknown brief subcommand") {
		t.Fatalf("expected an unknown-subcommand error, got: %v", err)
	}
}
