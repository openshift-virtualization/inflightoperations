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
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// crdDocument represents the minimal fields we need from a CRD YAML.
type crdDocument struct {
	Spec crdSpec `json:"spec"`
}

type crdSpec struct {
	Group    string       `json:"group"`
	Names    crdNames     `json:"names"`
	Versions []crdVersion `json:"versions"`
}

type crdNames struct {
	Kind   string `json:"kind"`
	Plural string `json:"plural"`
}

type crdVersion struct {
	Name   string `json:"name"`
	Served bool   `json:"served"`
}

// readOwnedCRDs scans a directory of CRD YAML files and returns
// CRDDescription entries for each, suitable for the CSV's owned CRDs section.
func readOwnedCRDs(dir string) ([]CRDDescription, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading CRD directory %s: %w", dir, err)
	}
	owned := make([]CRDDescription, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		desc, err := readCRD(path)
		if err != nil {
			return nil, err
		}
		owned = append(owned, desc)
	}
	return owned, nil
}

// readCRD reads a single CRD YAML file and extracts its name, kind,
// and the first served version.
func readCRD(path string) (CRDDescription, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CRDDescription{}, fmt.Errorf("reading CRD file %s: %w", path, err)
	}
	var doc crdDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return CRDDescription{}, fmt.Errorf("parsing CRD file %s: %w", path, err)
	}
	version := ""
	for _, v := range doc.Spec.Versions {
		if v.Served {
			version = v.Name
			break
		}
	}
	name := doc.Spec.Names.Plural + "." + doc.Spec.Group
	return CRDDescription{
		Name:    name,
		Version: version,
		Kind:    doc.Spec.Names.Kind,
	}, nil
}
