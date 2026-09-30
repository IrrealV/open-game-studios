package toolcheck

import (
	"context"
	"fmt"
	"strings"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusWarning Status = "warning"
	StatusFailure Status = "failure"
)

type Severity string

const (
	SeverityRequired Severity = "required"
	SeverityOptional Severity = "optional"
)

type Metadata struct {
	ID          string
	Name        string
	Aliases     []string
	Commands    []string
	Severity    Severity
	Required    bool
	Remediation string
}

type Result struct {
	ToolID      string
	ToolName    string
	Status      Status
	Severity    Severity
	Required    bool
	Reason      string
	Attempted   string
	Remediation string
}

func (r Result) Blocking() bool {
	if r.Status != StatusFailure {
		return false
	}
	if r.Severity == SeverityOptional || !r.Required && r.Severity == SeverityOptional {
		return false
	}
	return true
}

func (r Result) String() string {
	parts := []string{fmt.Sprintf("%s: %s", r.ToolID, r.Status)}
	if strings.TrimSpace(r.Reason) != "" {
		parts = append(parts, r.Reason)
	}
	if strings.TrimSpace(r.Attempted) != "" {
		parts = append(parts, "attempted "+r.Attempted)
	}
	if strings.TrimSpace(r.Remediation) != "" {
		parts = append(parts, r.Remediation)
	}
	return strings.Join(parts, " — ")
}

type Check interface {
	Metadata() Metadata
	Run(ctx context.Context, runner CommandRunner) Result
}

type Selection struct {
	Mode    SelectionMode
	ToolIDs []string
}

type SelectionMode string

const (
	SelectionModeDefault  SelectionMode = ""
	SelectionModeAll      SelectionMode = "all"
	SelectionModeNone     SelectionMode = "none"
	SelectionModeSelected SelectionMode = "selected"
)

func SelectAll() Selection { return Selection{Mode: SelectionModeAll} }

func SelectNone() Selection { return Selection{Mode: SelectionModeNone} }

func SelectTools(ids ...string) Selection {
	return Selection{Mode: SelectionModeSelected, ToolIDs: append([]string{}, ids...)}
}

func HasBlocking(results []Result) bool {
	for _, result := range results {
		if result.Blocking() {
			return true
		}
	}
	return false
}

func BlockingResults(results []Result) []Result {
	out := make([]Result, 0)
	for _, result := range results {
		if result.Blocking() {
			out = append(out, result)
		}
	}
	return out
}
