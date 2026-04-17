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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rulesDir string
	command  = &cobra.Command{
		Use:   "ruleset-bundler",
		Short: "Bundle all OperationRuleSet files into a single YAML document",
		Long: "Scans OperationRuleSet YAML files in the rules directory and " +
			"concatenates them into a single multi-document YAML stream.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	command.Flags().StringVar(&rulesDir, "rules-dir",
		"rules", "Directory containing OperationRuleSet YAML files")
}

func main() {
	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var paths []string

	err := filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking rules directory %s: %w", rulesDir, err)
	}

	sort.Strings(paths)

	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		data = bytes.TrimSpace(data)

		if _, err := fmt.Fprintln(os.Stdout, "---"); err != nil {
			return err
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
		if i < len(paths)-1 {
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		return err
	}

	return nil
}
