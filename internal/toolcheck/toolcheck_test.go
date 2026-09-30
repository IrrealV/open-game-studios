package toolcheck

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type stubCheck struct {
	meta   Metadata
	result Result
	calls  *int
}

func (c stubCheck) Metadata() Metadata { return c.meta }
func (c stubCheck) Run(context.Context, CommandRunner) Result {
	if c.calls != nil {
		(*c.calls)++
	}
	return c.result
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type stubCommandRunner struct {
	paths    map[string]string
	runs     map[string]CommandResult
	errs     map[string]error
	lookErrs map[string]error
	seen     []string
}

func (r *stubCommandRunner) LookPath(name string) (string, error) {
	if path, ok := r.paths[name]; ok {
		return path, nil
	}
	if err, ok := r.lookErrs[name]; ok {
		return "", err
	}
	return "", errors.New(name + " was not found on PATH")
}

func (r *stubCommandRunner) Run(_ context.Context, name string, args ...string) (CommandResult, error) {
	key := name + " " + strings.Join(args, " ")
	r.seen = append(r.seen, key)
	if err, ok := r.errs[key]; ok {
		return CommandResult{}, err
	}
	return r.runs[key], nil
}

func TestRegistryResolve_IndependentSelectionAndUnknownTools(t *testing.T) {
	registry := NewDefaultRegistry()
	listed := registry.List()
	ids := []string{listed[0].ID, listed[1].ID}
	if !reflect.DeepEqual(ids, []string{"blender", "godot"}) {
		t.Fatalf("default registry ids=%v, want blender/godot", ids)
	}

	future := stubCheck{meta: Metadata{ID: "maya", Name: "Maya"}}
	if err := registry.Register(future); err != nil {
		t.Fatalf("register future check: %v", err)
	}

	resolved := registry.Resolve(Selection{ToolIDs: []string{"godot"}})
	if len(resolved.Checks) != 1 || resolved.Checks[0].Metadata().ID != "godot" {
		t.Fatalf("expected only godot check, got %#v", resolved.Checks)
	}

	resolved = registry.Resolve(Selection{ToolIDs: []string{"unknown"}})
	if len(resolved.Results) != 1 || resolved.Results[0].Status != StatusFailure || !strings.Contains(resolved.Results[0].Reason, "unknown") {
		t.Fatalf("expected unknown-tool failure, got %#v", resolved.Results)
	}
}

func TestRegistryResolve_ExplicitSelectionModesAndAliases(t *testing.T) {
	registry := NewDefaultRegistry()

	all := registry.Resolve(SelectAll())
	if len(all.Checks) != 2 {
		t.Fatalf("SelectAll resolved %d checks, want 2", len(all.Checks))
	}

	none := registry.Resolve(SelectNone())
	if len(none.Checks) != 0 || len(none.Results) != 0 {
		t.Fatalf("SelectNone resolved checks=%#v results=%#v, want empty", none.Checks, none.Results)
	}

	selectedEmpty := registry.Resolve(SelectTools())
	if len(selectedEmpty.Checks) != 0 || len(selectedEmpty.Results) != 0 {
		t.Fatalf("empty selected resolved checks=%#v results=%#v, want empty", selectedEmpty.Checks, selectedEmpty.Results)
	}

	alias := registry.Resolve(SelectTools("godot4"))
	if len(alias.Checks) != 1 || alias.Checks[0].Metadata().ID != "godot" {
		t.Fatalf("godot4 alias resolved %#v, want godot", alias.Checks)
	}
}

func TestRegistryRegister_AliasConflictIsAtomic(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(stubCheck{meta: Metadata{ID: "godot", Name: "Godot", Aliases: []string{"godot4"}}}); err != nil {
		t.Fatalf("register godot: %v", err)
	}

	err := registry.Register(stubCheck{meta: Metadata{ID: "other", Name: "Other", Aliases: []string{"godot4"}}})
	if err == nil {
		t.Fatal("expected alias conflict error")
	}
	if _, ok := registry.Get("other"); ok {
		t.Fatal("conflicting check was registered despite alias conflict")
	}
	if resolved := registry.Resolve(SelectTools("other")); len(resolved.Results) != 1 || resolved.Results[0].Status != StatusFailure {
		t.Fatalf("other resolved after failed registration: %#v", resolved)
	}
}

func TestToolProbes_ClassifyResultsWithStubRunner(t *testing.T) {
	tests := []struct {
		name       string
		check      Check
		runner     *stubCommandRunner
		wantStatus Status
		wantRun    string
		wantText   string
	}{
		{
			name:       "godot success falls back to godot4",
			check:      NewGodotCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"godot4": `C:\Godot\godot4.exe`}, runs: map[string]CommandResult{"godot4 --version": {Stdout: "Godot Engine v4.2\n"}}},
			wantStatus: StatusSuccess,
			wantRun:    "godot4 --version",
			wantText:   "Godot Engine",
		},
		{
			name:       "godot 3 fails because Godot 4 is required",
			check:      NewGodotCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"godot": "/usr/bin/godot"}, runs: map[string]CommandResult{"godot --version": {Stdout: "Godot Engine v3.5.3"}}},
			wantStatus: StatusFailure,
			wantRun:    "godot --version",
			wantText:   "Godot 4 is required",
		},
		{
			name:       "godot 3 then godot4 succeeds",
			check:      NewGodotCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"godot": "/usr/bin/godot", "godot4": "/usr/bin/godot4"}, runs: map[string]CommandResult{"godot --version": {Stdout: "Godot Engine v3.5.3"}, "godot4 --version": {Stdout: "Godot Engine v4.2.2"}}},
			wantStatus: StatusSuccess,
			wantRun:    "godot4 --version",
			wantText:   "Godot Engine v4.2.2",
		},
		{
			name:       "godot malformed version fails",
			check:      NewGodotCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"godot": "/usr/bin/godot"}, runs: map[string]CommandResult{"godot --version": {Stdout: "custom engine"}}},
			wantStatus: StatusFailure,
			wantRun:    "godot --version",
			wantText:   "could not parse",
		},
		{
			name:       "godot empty version warns",
			check:      NewGodotCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"godot": "/usr/bin/godot"}, runs: map[string]CommandResult{"godot --version": {Stdout: ""}}},
			wantStatus: StatusWarning,
			wantRun:    "godot --version",
			wantText:   "did not print a version",
		},
		{
			name:       "godot missing fails with path guidance",
			check:      NewGodotCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{}, runs: map[string]CommandResult{}},
			wantStatus: StatusFailure,
			wantText:   "PATH",
		},
		{
			name:       "blender success",
			check:      NewBlenderCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"blender": "/usr/bin/blender"}, runs: map[string]CommandResult{"blender --version": {Stdout: "Blender 4.1.0\n"}}},
			wantStatus: StatusSuccess,
			wantRun:    "blender --version",
			wantText:   "Blender 4.1.0",
		},
		{
			name:       "blender incomplete version warns",
			check:      NewBlenderCheck(),
			runner:     &stubCommandRunner{paths: map[string]string{"blender": "/usr/bin/blender"}, runs: map[string]CommandResult{"blender --version": {Stdout: ""}}},
			wantStatus: StatusWarning,
			wantRun:    "blender --version",
			wantText:   "incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.check.Run(context.Background(), tt.runner)
			if result.Status != tt.wantStatus {
				t.Fatalf("status=%s want=%s; result=%#v", result.Status, tt.wantStatus, result)
			}
			if tt.wantRun != "" && !containsString(tt.runner.seen, tt.wantRun) {
				t.Fatalf("expected run %q, got %v", tt.wantRun, tt.runner.seen)
			}
			if tt.wantText != "" && !strings.Contains(result.String(), tt.wantText) {
				t.Fatalf("expected result to include %q, got %s", tt.wantText, result.String())
			}
		})
	}
}

func TestGodotProbe_AggregatesRunAndFallbackDiagnostics(t *testing.T) {
	runner := &stubCommandRunner{
		paths:    map[string]string{"godot": "/usr/bin/godot"},
		errs:     map[string]error{"godot --version": errors.New("permission denied")},
		lookErrs: map[string]error{"godot4": errors.New("godot4 was not found on PATH")},
	}

	result := NewGodotCheck().Run(context.Background(), runner)
	if result.Status != StatusFailure {
		t.Fatalf("status=%s want failure; result=%#v", result.Status, result)
	}
	for _, token := range []string{"godot run: permission denied", "godot4 lookup: godot4 was not found on PATH"} {
		if !strings.Contains(result.Reason, token) {
			t.Fatalf("expected diagnostic %q in %q", token, result.Reason)
		}
	}
	if strings.Index(result.Reason, "godot run") > strings.Index(result.Reason, "godot4 lookup") {
		t.Fatalf("expected godot run error to keep priority before fallback lookup, got %q", result.Reason)
	}
}

func TestBlockingResults_OptionalFailuresDoNotBlock(t *testing.T) {
	results := []Result{
		{ToolID: "optional", Status: StatusFailure, Severity: SeverityOptional, Required: false},
		{ToolID: "required", Status: StatusFailure, Severity: SeverityRequired, Required: true},
	}
	blocking := BlockingResults(results)
	if len(blocking) != 1 || blocking[0].ToolID != "required" {
		t.Fatalf("blocking=%#v, want only required failure", blocking)
	}
}

func TestRunner_NonFailFastAndFailFast(t *testing.T) {
	failingCalls := 0
	successCalls := 0
	registry := NewRegistry()
	_ = registry.Register(stubCheck{meta: Metadata{ID: "a", Name: "A"}, result: Result{ToolID: "a", Status: StatusFailure}, calls: &failingCalls})
	_ = registry.Register(stubCheck{meta: Metadata{ID: "b", Name: "B"}, result: Result{ToolID: "b", Status: StatusSuccess}, calls: &successCalls})

	runner := NewRunner(registry, &stubCommandRunner{})
	results := runner.Execute(context.Background(), Selection{ToolIDs: []string{"a", "b"}})
	if len(results) != 2 || failingCalls != 1 || successCalls != 1 {
		t.Fatalf("expected both checks to run, results=%#v calls=%d/%d", results, failingCalls, successCalls)
	}

	failingCalls, successCalls = 0, 0
	runner.FailFast = true
	results = runner.Execute(context.Background(), Selection{ToolIDs: []string{"a", "b"}})
	if len(results) != 1 || failingCalls != 1 || successCalls != 0 {
		t.Fatalf("expected fail-fast to stop after first failure, results=%#v calls=%d/%d", results, failingCalls, successCalls)
	}
}
