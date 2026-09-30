package integrations

import "open-game-studios/internal/assets"

type Integration struct {
	ID                string
	Label             string
	Category          string
	Description       string
	DefaultSelected   bool
	DiagnosticToolIDs []string
	Hints             []string
	Consent           []ConsentRequirement
	AssetAdapter      *assets.AdapterCapability
}

type ConsentRequirement struct {
	ID           string
	Label        string
	RequiredText string
}

type OptionalIntegrationSelection struct {
	ID      string          `json:"id"`
	Consent map[string]bool `json:"consent,omitempty"`
}

type Catalog interface {
	List() []Integration
	Get(id string) (Integration, bool)
}
