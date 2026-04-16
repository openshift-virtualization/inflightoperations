/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

const rulesDir = "../../rules"

func TestCollectTargets_SyntheticRules(t *testing.T) {
	dir := t.TempDir()

	writeRule(t, dir, "a.yaml", "", "pods")
	writeRule(t, dir, "b.yaml", "apps", "deployments")
	writeRule(t, dir, "c.yaml", "apps", "statefulsets")
	writeRule(t, dir, "d.yaml", "", "pods") // duplicate

	targets, err := collectTargets(dir)
	if err != nil {
		t.Fatalf("collectTargets: %v", err)
	}

	assertResources(t, targets, "", []string{"pods"})
	assertResources(t, targets, "apps", []string{"deployments", "statefulsets"})
}

func TestCollectTargets_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()

	writeRule(t, dir, "a.yaml", "apps", "deployments")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644); err != nil {
		t.Fatal(err)
	}

	targets, err := collectTargets(dir)
	if err != nil {
		t.Fatalf("collectTargets: %v", err)
	}

	if len(targets) != 1 {
		t.Errorf("expected 1 group, got %d", len(targets))
	}
}

func TestCollectTargets_Subdirectories(t *testing.T) {
	dir := t.TempDir()

	writeRule(t, dir, "a.yaml", "", "pods")

	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeRule(t, sub, "b.yaml", "apps", "deployments")

	targets, err := collectTargets(dir)
	if err != nil {
		t.Fatalf("collectTargets: %v", err)
	}

	assertResources(t, targets, "", []string{"pods"})
	assertResources(t, targets, "apps", []string{"deployments"})
}

func TestBuildClusterRole_Structure(t *testing.T) {
	targets := map[string][]string{
		"":     {"pods"},
		"apps": {"deployments", "statefulsets"},
	}

	role := buildClusterRole("test-role", targets)

	if role.APIVersion != "rbac.authorization.k8s.io/v1" {
		t.Errorf("unexpected apiVersion: %s", role.APIVersion)
	}
	if role.Kind != "ClusterRole" {
		t.Errorf("unexpected kind: %s", role.Kind)
	}
	if role.Metadata.Name != "test-role" {
		t.Errorf("unexpected name: %s", role.Metadata.Name)
	}
	if len(role.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(role.Rules))
	}

	// Core group sorts first
	if role.Rules[0].APIGroups[0] != "" {
		t.Errorf("expected core group first, got %q", role.Rules[0].APIGroups[0])
	}
	if role.Rules[1].APIGroups[0] != "apps" {
		t.Errorf("expected apps group second, got %q", role.Rules[1].APIGroups[0])
	}

	for _, rule := range role.Rules {
		if !slices.Equal(rule.Verbs, []string{"get", "list", "watch"}) {
			t.Errorf("unexpected verbs for group %q: %v", rule.APIGroups[0], rule.Verbs)
		}
	}
}

// TestAllRuleTargetsAppearInRole walks the real rules/ directory independently,
// parses every OperationRuleSet YAML, and verifies that each target (group, resource)
// appears in the ClusterRole produced by the generator.
func TestAllRuleTargetsAppearInRole(t *testing.T) {
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skipf("rules directory not available: %v", err)
	}

	// Step 1: independently collect every (group, resource) from rule files.
	type target struct {
		group    string
		resource string
		file     string
	}
	var expected []target

	err := filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc struct {
			Spec struct {
				Target struct {
					Group    string `json:"group"`
					Resource string `json:"resource"`
				} `json:"target"`
			} `json:"spec"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return err
		}
		if doc.Spec.Target.Resource == "" {
			return nil
		}
		expected = append(expected, target{
			group:    doc.Spec.Target.Group,
			resource: doc.Spec.Target.Resource,
			file:     path,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking rules directory: %v", err)
	}

	if len(expected) == 0 {
		t.Fatal("found no rule targets — something is wrong")
	}

	// Step 2: run the generator.
	targets, err := collectTargets(rulesDir)
	if err != nil {
		t.Fatalf("collectTargets: %v", err)
	}
	role := buildClusterRole("watch-role", targets)

	// Step 3: index the generated role for fast lookup.
	roleIndex := make(map[string]map[string]bool)
	for _, rule := range role.Rules {
		for _, group := range rule.APIGroups {
			if roleIndex[group] == nil {
				roleIndex[group] = make(map[string]bool)
			}
			for _, res := range rule.Resources {
				roleIndex[group][res] = true
			}
		}
	}

	// Step 4: verify every independently-parsed target is present.
	for _, tgt := range expected {
		if !roleIndex[tgt.group][tgt.resource] {
			t.Errorf("rule file %s: target {group:%q resource:%q} missing from generated role",
				tgt.file, tgt.group, tgt.resource)
		}
	}
}

func writeRule(t *testing.T, dir, name, group, resource string) {
	t.Helper()
	content := "apiVersion: ifo.kubevirt.io/v1alpha1\nkind: OperationRuleSet\n" +
		"metadata:\n  name: test\nspec:\n  target:\n    group: " + quote(group) +
		"\n    version: v1\n    resource: " + resource +
		"\n  rules:\n    - operation: Test\n      expression: \"true\"\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func quote(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

func assertResources(t *testing.T, targets map[string][]string, group string, expected []string) {
	t.Helper()
	got, ok := targets[group]
	if !ok {
		t.Errorf("group %q not found in targets", group)
		return
	}
	if !slices.Equal(got, expected) {
		t.Errorf("group %q: got %v, want %v", group, got, expected)
	}
}
