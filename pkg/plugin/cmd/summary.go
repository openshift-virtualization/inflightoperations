package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/output"
	"github.com/spf13/cobra"
)

// NewSummaryCommand creates the "summary" subcommand.
func NewSummaryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Show aggregate summary of InFlightOperations",
		RunE:  runSummary,
	}
}

func runSummary(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	// Always fetch all phases for summary counts.
	opts := buildListOptions()
	// Override phase filter — show both active and completed.
	opts.FieldSelector = removeFieldSelector(opts.FieldSelector, "status.phase")

	ifos, err := c.List(context.TODO(), opts)
	if err != nil {
		return err
	}

	if len(ifos) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No in-flight operations found.")
		return nil
	}

	color := output.NewColorWriter(globalFlags.Color, globalFlags.NoColor)

	printComponentSummary(os.Stdout, ifos, color)
	_, _ = fmt.Fprintln(os.Stdout)
	printOperationSummary(os.Stdout, ifos, color)

	return nil
}

func printComponentSummary(w *os.File, ifos []api.InFlightOperation, color *output.ColorWriter) {
	type counts struct {
		active    int
		completed int
	}
	byComponent := make(map[string]*counts)
	var order []string

	for i := range ifos {
		comp := ifos[i].Spec.Component
		if comp == "" {
			comp = "(none)"
		}
		c, ok := byComponent[comp]
		if !ok {
			c = &counts{}
			byComponent[comp] = c
			order = append(order, comp)
		}
		if ifos[i].Status.Phase == api.OperationPhaseActive {
			c.active++
		} else {
			c.completed++
		}
	}
	slices.Sort(order)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, color.Bold("COMPONENT\tACTIVE\tCOMPLETED\tTOTAL"))
	totalActive := 0
	totalCompleted := 0
	for _, comp := range order {
		c := byComponent[comp]
		totalActive += c.active
		totalCompleted += c.completed
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n",
			comp, c.active, c.completed, c.active+c.completed)
	}
	_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n",
		color.Bold("Total"), totalActive, totalCompleted, totalActive+totalCompleted)
	_ = tw.Flush()
}

func printOperationSummary(w *os.File, ifos []api.InFlightOperation, color *output.ColorWriter) {
	// Group active operations by kind.
	byKind := make(map[string]map[string]int)
	var kindOrder []string

	for i := range ifos {
		if ifos[i].Status.Phase != api.OperationPhaseActive {
			continue
		}
		kind := ifos[i].Spec.Subject.Kind
		op := ifos[i].Spec.Operation
		if _, ok := byKind[kind]; !ok {
			byKind[kind] = make(map[string]int)
			kindOrder = append(kindOrder, kind)
		}
		byKind[kind][op]++
	}
	slices.Sort(kindOrder)

	if len(kindOrder) == 0 {
		return
	}

	_, _ = fmt.Fprintln(w, color.Bold("ACTIVE OPERATIONS BY KIND:"))
	for _, kind := range kindOrder {
		ops := byKind[kind]
		var parts []string
		// Sort operation names.
		var opNames []string
		for name := range ops {
			opNames = append(opNames, name)
		}
		slices.Sort(opNames)
		for _, name := range opNames {
			parts = append(parts, fmt.Sprintf("%s(%d)", name, ops[name]))
		}
		_, _ = fmt.Fprintf(w, "  %-28s %s\n", kind+":", strings.Join(parts, ", "))
	}
}

func removeFieldSelector(selector, field string) string {
	if selector == "" {
		return ""
	}
	parts := strings.Split(selector, ",")
	var filtered []string
	for _, p := range parts {
		if !strings.HasPrefix(p, field+"=") {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",")
}
