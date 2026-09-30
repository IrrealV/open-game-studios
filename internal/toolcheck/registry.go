package toolcheck

import (
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	checks  map[string]Check
	aliases map[string]string
}

func NewRegistry() *Registry {
	return &Registry{checks: map[string]Check{}, aliases: map[string]string{}}
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(NewGodotCheck())
	_ = registry.Register(NewBlenderCheck())
	return registry
}

func (r *Registry) Register(check Check) error {
	if check == nil {
		return fmt.Errorf("tool check is nil")
	}
	meta := check.Metadata()
	id := normalizeToolID(meta.ID)
	if id == "" {
		return fmt.Errorf("tool check id is required")
	}
	if strings.TrimSpace(meta.Name) == "" {
		return fmt.Errorf("tool check %q name is required", id)
	}
	if _, exists := r.checks[id]; exists {
		return fmt.Errorf("tool check %q already registered", id)
	}
	if canonical, exists := r.aliases[id]; exists {
		return fmt.Errorf("tool check id %q conflicts with alias for %q", id, canonical)
	}
	aliases := make([]string, 0, len(meta.Aliases))
	for _, rawAlias := range meta.Aliases {
		alias := normalizeToolID(rawAlias)
		if alias == "" || alias == id {
			continue
		}
		if _, exists := r.checks[alias]; exists {
			return fmt.Errorf("tool check alias %q conflicts with registered id", alias)
		}
		if existing, exists := r.aliases[alias]; exists && existing != id {
			return fmt.Errorf("tool check alias %q already registered for %q", alias, existing)
		}
		aliases = append(aliases, alias)
	}
	r.checks[id] = check
	for _, alias := range aliases {
		r.aliases[alias] = id
	}
	return nil
}

func (r *Registry) List() []Metadata {
	out := make([]Metadata, 0, len(r.checks))
	for _, check := range r.checks {
		out = append(out, check.Metadata())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Get(id string) (Check, bool) {
	check, ok := r.checks[r.CanonicalID(id)]
	return check, ok
}

func (r *Registry) CanonicalID(id string) string {
	normalized := normalizeToolID(id)
	if canonical, ok := r.aliases[normalized]; ok {
		return canonical
	}
	return normalized
}

type Resolved struct {
	Checks  []Check
	Results []Result
}

func (r *Registry) Resolve(selection Selection) Resolved {
	ids := selection.ToolIDs
	mode := selection.Mode
	if mode == SelectionModeDefault {
		if len(ids) == 0 {
			mode = SelectionModeAll
		} else {
			mode = SelectionModeSelected
		}
	}
	if mode == SelectionModeNone {
		return Resolved{}
	}
	if mode == SelectionModeAll {
		ids = make([]string, 0, len(r.checks))
		for id := range r.checks {
			ids = append(ids, id)
		}
	}

	seen := map[string]struct{}{}
	checks := make([]Check, 0, len(ids))
	results := make([]Result, 0)
	for _, raw := range ids {
		id := r.CanonicalID(raw)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		check, ok := r.checks[id]
		if !ok {
			results = append(results, Result{ToolID: id, ToolName: id, Status: StatusFailure, Severity: SeverityRequired, Required: true, Reason: "unknown tool selection", Attempted: id, Remediation: "Use a registered tool id from env-check output or add a dedicated tool check registration."})
			continue
		}
		checks = append(checks, check)
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Metadata().ID < checks[j].Metadata().ID })
	return Resolved{Checks: checks, Results: results}
}

func normalizeToolID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
