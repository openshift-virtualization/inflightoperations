package output

import (
	"encoding/json"
	"fmt"
	"io"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	"sigs.k8s.io/yaml"
)

// PrintJSON writes IFOs as a JSON array.
func PrintJSON(w io.Writer, ifos []api.InFlightOperation) error {
	data, err := json.MarshalIndent(ifos, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
}

// PrintYAML writes IFOs as a YAML document.
func PrintYAML(w io.Writer, ifos []api.InFlightOperation) error {
	data, err := yaml.Marshal(ifos)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// PrintJSONSingle writes a single IFO as JSON.
func PrintJSONSingle(w io.Writer, ifo *api.InFlightOperation) error {
	data, err := json.MarshalIndent(ifo, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
}

// PrintYAMLSingle writes a single IFO as YAML.
func PrintYAMLSingle(w io.Writer, ifo *api.InFlightOperation) error {
	data, err := yaml.Marshal(ifo)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	_, err = w.Write(data)
	return err
}
