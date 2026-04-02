package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
)

// TablePrinter writes IFOs as a formatted table.
type TablePrinter struct {
	Color *ColorWriter
	Wide  bool
}

// PrintList writes a table of IFOs to w.
func (p *TablePrinter) PrintList(w io.Writer, ifos []api.InFlightOperation) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if p.Wide {
		_, _ = fmt.Fprintln(tw,
			p.Color.Bold("SUBJECT KIND\tNAMESPACE\tSUBJECT NAME\tOPERATION\tPHASE\tCOMPONENT\tRULESET\tAGE"))
	} else {
		_, _ = fmt.Fprintln(tw,
			p.Color.Bold("SUBJECT KIND\tNAMESPACE\tSUBJECT NAME\tOPERATION\tPHASE\tCOMPONENT\tAGE"))
	}

	for i := range ifos {
		p.printRow(tw, &ifos[i])
	}
	_ = tw.Flush()
}

func (p *TablePrinter) printRow(w io.Writer, ifo *api.InFlightOperation) {
	phase := string(ifo.Status.Phase)
	if ifo.Status.Phase == api.OperationPhaseActive {
		phase = p.Color.Green(phase)
	} else {
		phase = p.Color.Dim(phase)
	}

	operation := colorizeOp(p.Color, ifo.Spec.Operation)
	age := FormatAge(ifo.CreationTimestamp.Time)

	component := ifo.Spec.Component
	if component == "" {
		component = "-"
	}

	if p.Wide {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			ifo.Spec.Subject.Kind,
			defaultString(ifo.Spec.Subject.Namespace, "-"),
			ifo.Spec.Subject.Name,
			operation,
			phase,
			component,
			ifo.Spec.RuleSet,
			age,
		)
	} else {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			ifo.Spec.Subject.Kind,
			defaultString(ifo.Spec.Subject.Namespace, "-"),
			ifo.Spec.Subject.Name,
			operation,
			phase,
			component,
			age,
		)
	}
}

// PrintDetail writes a detailed view of a single IFO.
func (p *TablePrinter) PrintDetail(w io.Writer, ifo *api.InFlightOperation) {
	_, _ = fmt.Fprintf(w, "%s %s\n", p.Color.Bold("Name:"), ifo.Name)
	_, _ = fmt.Fprintf(w, "%s %s\n", p.Color.Bold("Operation:"), ifo.Spec.Operation)
	_, _ = fmt.Fprintf(w, "%s %s\n", p.Color.Bold("Phase:"), ifo.Status.Phase)
	_, _ = fmt.Fprintf(w, "%s %s\n",
		p.Color.Bold("Component:"), defaultString(ifo.Spec.Component, "(none)"))
	_, _ = fmt.Fprintf(w, "%s %s\n", p.Color.Bold("RuleSet:"), ifo.Spec.RuleSet)
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, p.Color.Bold("Subject:"))
	_, _ = fmt.Fprintf(w, "  Kind:       %s\n", ifo.Spec.Subject.Kind)
	_, _ = fmt.Fprintf(w, "  Name:       %s\n", ifo.Spec.Subject.Name)
	_, _ = fmt.Fprintf(w, "  Namespace:  %s\n",
		defaultString(ifo.Spec.Subject.Namespace, "(cluster-scoped)"))
	_, _ = fmt.Fprintf(w, "  APIVersion: %s\n", ifo.Spec.Subject.APIVersion)
	_, _ = fmt.Fprintf(w, "  UID:        %s\n", ifo.Spec.Subject.UID)
	if len(ifo.Spec.Subject.OwnerReferences) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, p.Color.Bold("Owner References:"))
		for _, ref := range ifo.Spec.Subject.OwnerReferences {
			_, _ = fmt.Fprintf(w, "  - %s/%s (uid: %s)\n", ref.Kind, ref.Name, ref.UID)
		}
	}

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, p.Color.Bold("Status:"))
	_, _ = fmt.Fprintf(w, "  Created:           %s (%s ago)\n",
		ifo.CreationTimestamp.Format(time.RFC3339), FormatAge(ifo.CreationTimestamp.Time))
	if ifo.Status.LastDetected != nil {
		_, _ = fmt.Fprintf(w, "  Last Detected:     %s (%s ago)\n",
			ifo.Status.LastDetected.Format(time.RFC3339), FormatAge(ifo.Status.LastDetected.Time))
	}
	if ifo.Status.Completed != nil {
		_, _ = fmt.Fprintf(w, "  Completed:         %s (%s ago)\n",
			ifo.Status.Completed.Format(time.RFC3339), FormatAge(ifo.Status.Completed.Time))
	}
	if len(ifo.Status.DetectedBy) > 0 {
		_, _ = fmt.Fprintf(w, "  Detected By:       %s\n",
			strings.Join(ifo.Status.DetectedBy, ", "))
	}
	if ifo.Status.SubjectGeneration > 0 {
		_, _ = fmt.Fprintf(w, "  Subject Gen:       %d\n", ifo.Status.SubjectGeneration)
	}

	if len(ifo.Labels) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, p.Color.Bold("Labels:"))
		for k, v := range ifo.Labels {
			_, _ = fmt.Fprintf(w, "  %s=%s\n", k, v)
		}
	}
}

func isErrorOperation(op string) bool {
	switch op {
	case "Failed", "Failing", "BuildFailed", "Lost":
		return true
	}
	return false
}

func isWarningOperation(op string) bool {
	switch op {
	case "Degraded", "UpdatingDegraded", "ReconcilingDegraded",
		"NodeDegraded", "RenderDegraded", "NotUpgradeable":
		return true
	}
	return false
}

func colorizeOp(c *ColorWriter, op string) string {
	if isErrorOperation(op) {
		return c.Red(op)
	}
	if isWarningOperation(op) {
		return c.Yellow(op)
	}
	return op
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// FormatAge returns a human-readable duration string.
func FormatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}
