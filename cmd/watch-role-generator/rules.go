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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

type ruleSetDocument struct {
	Spec ruleSetSpec `json:"spec"`
}

type ruleSetSpec struct {
	Target ruleSetTarget `json:"target"`
}

type ruleSetTarget struct {
	Group    string `json:"group"`
	Resource string `json:"resource"`
}

func collectTargets(rulesDir string) (map[string][]string, error) {
	grouped := make(map[string]map[string]struct{})

	err := filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		var doc ruleSetDocument
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		if doc.Spec.Target.Resource == "" {
			return nil
		}

		group := doc.Spec.Target.Group
		if grouped[group] == nil {
			grouped[group] = make(map[string]struct{})
		}
		grouped[group][doc.Spec.Target.Resource] = struct{}{}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking rules directory %s: %w", rulesDir, err)
	}

	result := make(map[string][]string, len(grouped))
	for group, resources := range grouped {
		sorted := make([]string, 0, len(resources))
		for r := range resources {
			sorted = append(sorted, r)
		}
		sort.Strings(sorted)
		result[group] = sorted
	}

	return result, nil
}
