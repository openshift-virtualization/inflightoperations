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
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// rbacDocument represents a minimal ClusterRole or Role YAML.
type rbacDocument struct {
	Rules []PolicyRule `json:"rules"`
}

// readRBACRules reads a ClusterRole or Role YAML file and returns
// the policy rules it contains.
func readRBACRules(path string) ([]PolicyRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading RBAC file %s: %w", path, err)
	}
	var doc rbacDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing RBAC file %s: %w", path, err)
	}
	return doc.Rules, nil
}

// metricsAuthRules returns the static RBAC rules for metrics endpoint
// authentication. These originate from the kustomize metrics patch and
// are not present in any controller-gen generated RBAC file.
func metricsAuthRules() []PolicyRule {
	return []PolicyRule{
		{
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"tokenreviews"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"authorization.k8s.io"},
			Resources: []string{"subjectaccessreviews"},
			Verbs:     []string{"create"},
		},
	}
}
