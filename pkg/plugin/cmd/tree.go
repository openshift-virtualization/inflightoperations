package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	pluginclient "github.com/openshift-virtualization/inflightoperations/pkg/plugin/client"
	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/model"
	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/output"
	"github.com/spf13/cobra"
)

var treeFor string

// NewTreeCommand creates the "tree" subcommand.
func NewTreeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Show InFlightOperations as a grouped tree",
		RunE:  runTree,
	}
	cmd.Flags().StringVar(&treeFor, "for", "", "Focus on a specific resource (Kind/name)")
	return cmd
}

func runTree(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	var ifos []api.InFlightOperation

	if treeFor != "" {
		ifos, err = listForResource(ctx, c)
	} else {
		ifos, err = c.List(ctx, buildListOptions())
	}
	if err != nil {
		return err
	}

	forest := model.BuildForest(ifos, model.DefaultCorrelators())

	color := output.NewColorWriter(globalFlags.Color, globalFlags.NoColor)

	switch globalFlags.Output {
	case OutputJSON:
		return output.PrintJSON(os.Stdout, ifos)
	case OutputYAML:
		return output.PrintYAML(os.Stdout, ifos)
	default:
		printer := &output.TreePrinter{Color: color}
		if treeFor != "" && len(forest.Roots) == 1 {
			printer.PrintTree(os.Stdout, forest.Roots[0])
		} else {
			printer.PrintForest(os.Stdout, forest)
		}
		return nil
	}
}

// listForResource queries IFOs for a specific resource and its children.
func listForResource(ctx context.Context, c *pluginclient.IFOClient) ([]api.InFlightOperation, error) {
	kind, name, err := parseKindName(treeFor)
	if err != nil {
		return nil, err
	}

	// Find IFOs for the target resource.
	opts := buildListOptions()
	labelParts := []string{
		fmt.Sprintf("%s=%s", api.LabelSubjectKind, kind),
		fmt.Sprintf("%s=%s", api.LabelSubjectName, name),
	}
	if opts.LabelSelector != "" {
		opts.LabelSelector += "," + strings.Join(labelParts, ",")
	} else {
		opts.LabelSelector = strings.Join(labelParts, ",")
	}

	rootIFOs, err := c.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(rootIFOs) == 0 {
		return nil, nil
	}

	// Get the subject UID to find children via owner-uid label.
	subjectUID := rootIFOs[0].Spec.Subject.UID
	if subjectUID == "" {
		return rootIFOs, nil
	}

	childOpts := buildListOptions()
	childLabelParts := []string{
		fmt.Sprintf("%s=%s", api.LabelOwnerUID, subjectUID),
	}
	if childOpts.LabelSelector != "" {
		childOpts.LabelSelector += "," + strings.Join(childLabelParts, ",")
	} else {
		childOpts.LabelSelector = strings.Join(childLabelParts, ",")
	}

	childIFOs, err := c.List(ctx, childOpts)
	if err != nil {
		return nil, err
	}

	// Recursively find grandchildren.
	allIFOs := append(rootIFOs, childIFOs...)
	for _, child := range childIFOs {
		uid := child.Spec.Subject.UID
		if uid == "" {
			continue
		}
		gcOpts := buildListOptions()
		gcOpts.LabelSelector = fmt.Sprintf("%s=%s", api.LabelOwnerUID, uid)
		gcIFOs, err := c.List(ctx, gcOpts)
		if err != nil {
			return nil, err
		}
		allIFOs = append(allIFOs, gcIFOs...)
	}

	return allIFOs, nil
}

func parseKindName(s string) (kind, name string, err error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("--for must be in format Kind/name (e.g., VirtualMachine/my-vm)")
	}
	return parts[0], parts[1], nil
}
