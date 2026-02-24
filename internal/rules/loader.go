package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// LoadRules reads and parses a rules file from the given path
func LoadRules(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	return LoadRulesFromBytes(data)
}

// LoadRulesFromBytes parses rules from raw YAML bytes
func LoadRulesFromBytes(data []byte) (*RuleSet, error) {
	var rf RuleSet
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("failed to parse rules file: %w", err)
	}

	if err := rf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rules file: %w", err)
	}

	return &rf, nil
}

// LoadRulesForGVK loads rules matching the provided GVK.
// If path is a file: loads and verifies GVK matches
// If path is a directory: scans for file matching GVK
func LoadRulesForGVK(path string, gvk schema.GroupVersionKind) (*RuleSet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access rules path: %w", err)
	}

	if !info.IsDir() {
		// Single file: load and verify GVK matches
		rf, err := LoadRules(path)
		if err != nil {
			return nil, err
		}

		if rf.GVK.ToSchemaGVK() != gvk {
			return nil, fmt.Errorf(
				"rules file GVK %s does not match resource GVK %s",
				rf.GVK.ToSchemaGVK(), gvk,
			)
		}

		return rf, nil
	}

	// Directory: scan for matching GVK
	return loadRulesFromDirectory(path, gvk)
}

// loadRulesFromDirectory scans a directory for rules files matching the given GVK
func loadRulesFromDirectory(dirPath string, targetGVK schema.GroupVersionKind) (*RuleSet, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules directory: %w", err)
	}

	var found *RuleSet
	var availableGVKs []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process YAML files
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dirPath, name)

		// Load and parse the rules file
		rulesFile, err := LoadRules(filePath)
		if err != nil {
			// Skip invalid files
			continue
		}

		fileGVK := rulesFile.GVK.ToSchemaGVK()
		availableGVKs = append(availableGVKs, fileGVK.String())

		if fileGVK == targetGVK {
			if found != nil {
				return nil, fmt.Errorf(
					"duplicate rules found for GVK %s",
					targetGVK,
				)
			}
			found = rulesFile
		}
	}

	if found == nil {
		return nil, fmt.Errorf(
			"no rules found for GVK %s. Available: %s",
			targetGVK,
			strings.Join(availableGVKs, ", "),
		)
	}

	return found, nil
}
