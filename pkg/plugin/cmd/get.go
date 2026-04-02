package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/output"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the "get" subcommand.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show details of a single InFlightOperation",
		Args:  cobra.ExactArgs(1),
		RunE:  runGet,
	}
}

func runGet(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	ifo, err := c.Get(context.TODO(), args[0])
	if err != nil {
		return fmt.Errorf("getting IFO: %w", err)
	}

	switch globalFlags.Output {
	case OutputJSON:
		return output.PrintJSONSingle(os.Stdout, ifo)
	case OutputYAML:
		return output.PrintYAMLSingle(os.Stdout, ifo)
	default:
		color := output.NewColorWriter()
		if globalFlags.NoColor {
			color.Enabled = false
		}
		printer := &output.TablePrinter{Color: color}
		printer.PrintDetail(os.Stdout, ifo)
		return nil
	}
}
