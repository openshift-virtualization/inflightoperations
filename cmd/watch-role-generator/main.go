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
	"sort"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type generatorFlags struct {
	rulesDir string
	roleName string
}

var (
	flags   generatorFlags
	command = &cobra.Command{
		Use:   "watch-role-generator",
		Short: "Generate the watch ClusterRole from OperationRuleSet files",
		Long: "Scans OperationRuleSet YAML files and generates a ClusterRole " +
			"with least-privilege RBAC rules for watching the targeted resources.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	command.Flags().StringVar(&flags.rulesDir, "rules-dir",
		"rules", "Directory containing OperationRuleSet YAML files")
	command.Flags().StringVar(&flags.roleName, "role-name",
		"watch-role", "Name for the generated ClusterRole")
}

func main() {
	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	targets, err := collectTargets(flags.rulesDir)
	if err != nil {
		return err
	}

	role := buildClusterRole(flags.roleName, targets)

	yamlBytes, err := yaml.Marshal(role)
	if err != nil {
		return fmt.Errorf("marshaling ClusterRole: %w", err)
	}

	if _, err := fmt.Fprint(os.Stdout, "---\n"); err != nil {
		return err
	}
	_, err = os.Stdout.Write(yamlBytes)
	return err
}

func buildClusterRole(name string, targets map[string][]string) ClusterRole {
	groups := make([]string, 0, len(targets))
	for g := range targets {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	rules := make([]PolicyRule, 0, len(groups))
	for _, group := range groups {
		rules = append(rules, PolicyRule{
			APIGroups: []string{group},
			Resources: targets[group],
			Verbs:     []string{"get", "list", "watch"},
		})
	}

	return ClusterRole{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Metadata:   ClusterRoleMeta{Name: name},
		Rules:      rules,
	}
}
