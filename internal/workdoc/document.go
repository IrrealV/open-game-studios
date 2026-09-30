package workdoc

import (
	"fmt"
	"os"
	"strings"
)

type Document struct {
	Path            string
	ProfileName     string
	PrimaryEngine   string
	Platform        string
	PersistenceMode string
	Raw             string
}

// CanonicalPlatform is the single supported platform token for the Game-Studio working document.
const CanonicalPlatform = "Pi only"

func LoadAndValidate(path string) (Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read working document: %w", err)
	}

	content := string(raw)
	doc := Document{Path: path, Raw: content}

	if err := validateRequiredSections(content); err != nil {
		return Document{}, err
	}

	profileLine, ok := findLineWithPrefix(content, "- User-facing profile count:")
	if !ok {
		return Document{}, fmt.Errorf("working document missing profile declaration")
	}
	profileName, ok := extractBacktickValue(profileLine)
	if !ok {
		return Document{}, fmt.Errorf("working document profile declaration must include backticks")
	}
	doc.ProfileName = strings.TrimSpace(profileName)

	primaryEngine, ok := findValueWithPrefix(content, "- Primary engine target (phase 1):")
	if !ok {
		return Document{}, fmt.Errorf("working document missing primary engine declaration")
	}
	doc.PrimaryEngine = strings.ToLower(strings.TrimSpace(primaryEngine))

	platform, ok := findValueWithPrefix(content, "- Platform:")
	if !ok {
		return Document{}, fmt.Errorf("working document missing platform declaration")
	}
	doc.Platform = strings.TrimSpace(platform)

	persistenceMode, ok := findValueWithPrefix(content, "- Persistence mode:")
	if !ok {
		return Document{}, fmt.Errorf("working document missing persistence declaration")
	}
	doc.PersistenceMode = strings.TrimSpace(persistenceMode)

	if strings.TrimSpace(doc.Platform) == "" {
		return Document{}, fmt.Errorf("working document platform declaration is empty: expected %q", CanonicalPlatform)
	}
	if !strings.EqualFold(doc.Platform, CanonicalPlatform) {
		return Document{}, fmt.Errorf("unsupported platform %q: expected %q (legacy values such as \"OpenCode only\" are no longer accepted)", doc.Platform, CanonicalPlatform)
	}

	if !strings.Contains(strings.ToLower(doc.PersistenceMode), "hybrid") {
		return Document{}, fmt.Errorf("unsupported persistence mode %q: expected hybrid", doc.PersistenceMode)
	}

	return doc, nil
}

func validateRequiredSections(content string) error {
	required := []string{
		"## 1) Vision",
		"## 2) Product Definition",
		"## 3) Non-Negotiable Constraints",
		"## 4) Architecture Direction (Working)",
		"## 6) CCGS Preservation Notes",
	}

	for _, marker := range required {
		if !strings.Contains(content, marker) {
			return fmt.Errorf("working document missing required section %q", marker)
		}
	}

	return nil
}

func findValueWithPrefix(content, prefix string) (string, bool) {
	line, ok := findLineWithPrefix(content, prefix)
	if !ok {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if value == "" {
		return "", false
	}

	return value, true
}

func findLineWithPrefix(content, prefix string) (string, bool) {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(line), true
		}
	}

	return "", false
}

func extractBacktickValue(line string) (string, bool) {
	start := strings.Index(line, "`")
	if start == -1 {
		return "", false
	}
	end := strings.Index(line[start+1:], "`")
	if end == -1 {
		return "", false
	}

	value := line[start+1 : start+1+end]
	if strings.TrimSpace(value) == "" {
		return "", false
	}

	return value, true
}
