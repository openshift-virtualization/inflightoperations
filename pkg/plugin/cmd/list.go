package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/output"
	"github.com/spf13/cobra"
)

// NewListCommand creates the "list" subcommand.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List InFlightOperations",
		RunE:    runList,
	}
	return cmd
}

func runList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	ifos, err := c.List(context.TODO(), buildListOptions())
	if err != nil {
		return err
	}

	if len(ifos) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No in-flight operations found.")
		return nil
	}

	// Sort by creation time (newest first).
	slices.SortFunc(ifos, func(a, b api.InFlightOperation) int {
		return b.CreationTimestamp.Compare(a.CreationTimestamp.Time)
	})

	switch globalFlags.Output {
	case OutputJSON:
		return output.PrintJSON(os.Stdout, ifos)
	case OutputYAML:
		return output.PrintYAML(os.Stdout, ifos)
	default:
		color := output.NewColorWriter(globalFlags.Color, globalFlags.NoColor)
		printer := &output.TablePrinter{
			Color: color,
			Wide:  globalFlags.Output == OutputWide,
		}
		printer.PrintList(os.Stdout, ifos)
		return nil
	}
}
