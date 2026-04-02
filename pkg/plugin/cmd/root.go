package cmd

import (
	"fmt"
	"os"
	"strings"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	pluginclient "github.com/openshift-virtualization/inflightoperations/pkg/plugin/client"
	"github.com/spf13/cobra"
)

// Output format constants.
const (
	OutputJSON  = "json"
	OutputYAML  = "yaml"
	OutputWide  = "wide"
	OutputTable = "table"
)

// GlobalFlags holds flags shared by all subcommands.
type GlobalFlags struct {
	Kubeconfig string
	Context    string
	AllPhases  bool
	Namespace  string
	Component  string
	Kind       string
	Operation  string
	Output     string
	NoColor    bool
	Selector   string
}

var globalFlags GlobalFlags

// NewRootCommand creates the root kubectl-ifo command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "kubectl-ifo",
		Short: "View and explore InFlightOperations",
		Long:  "kubectl-ifo shows active and completed InFlightOperations with grouping, filtering, and tree views.",
	}

	root.PersistentFlags().StringVar(&globalFlags.Kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	root.PersistentFlags().StringVar(&globalFlags.Context, "context", "", "Kubernetes context to use")
	pf := root.PersistentFlags()
	pf.BoolVarP(&globalFlags.AllPhases, "all-phases", "A", false,
		"Show completed IFOs too")
	pf.StringVarP(&globalFlags.Namespace, "namespace", "n", "",
		"Filter by subject namespace")
	pf.StringVarP(&globalFlags.Component, "component", "c", "",
		"Filter by component")
	pf.StringVarP(&globalFlags.Kind, "kind", "k", "",
		"Filter by subject kind")
	pf.StringVarP(&globalFlags.Operation, "operation", "o", "",
		"Filter by operation name")
	pf.StringVarP(&globalFlags.Output, "output", "O", "table",
		"Output format: table, wide, json, yaml")
	pf.BoolVar(&globalFlags.NoColor, "no-color", false,
		"Disable ANSI color output")
	pf.StringVarP(&globalFlags.Selector, "selector", "l", "",
		"Label selector (passthrough)")

	root.AddCommand(NewListCommand())
	root.AddCommand(NewTreeCommand())
	root.AddCommand(NewGetCommand())
	root.AddCommand(NewSummaryCommand())

	// Default to list when no subcommand is given.
	root.RunE = NewListCommand().RunE

	return root
}

// newClient creates an IFO client from the global flags.
func newClient() (*pluginclient.IFOClient, error) {
	return pluginclient.NewFromKubeconfig(globalFlags.Kubeconfig, globalFlags.Context)
}

// buildListOptions constructs server-side filters from global flags.
func buildListOptions() pluginclient.ListOptions {
	var labelParts []string
	var fieldParts []string

	if globalFlags.Component != "" {
		labelParts = append(labelParts, fmt.Sprintf("%s=%s", api.LabelComponent, globalFlags.Component))
	}
	if globalFlags.Kind != "" {
		labelParts = append(labelParts, fmt.Sprintf("%s=%s", api.LabelSubjectKind, globalFlags.Kind))
	}
	if globalFlags.Selector != "" {
		labelParts = append(labelParts, globalFlags.Selector)
	}

	if globalFlags.Namespace != "" {
		fieldParts = append(fieldParts, fmt.Sprintf("spec.subject.namespace=%s", globalFlags.Namespace))
	}
	if globalFlags.Operation != "" {
		fieldParts = append(fieldParts, fmt.Sprintf("spec.operation=%s", globalFlags.Operation))
	}
	if !globalFlags.AllPhases {
		fieldParts = append(fieldParts, "status.phase=Active")
	}

	return pluginclient.ListOptions{
		LabelSelector: strings.Join(labelParts, ","),
		FieldSelector: strings.Join(fieldParts, ","),
	}
}

// Execute runs the root command.
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
