package output

import (
	"fmt"
	"io"

	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/model"
)

const (
	treeBranch = "├── "
	treeCorner = "└── "
	treeBar    = "│   "
	treeBlank  = "    "
)

// TreePrinter renders a forest of IFO nodes as an indented tree.
type TreePrinter struct {
	Color *ColorWriter
}

// PrintForest renders the entire forest grouped by component.
func (p *TreePrinter) PrintForest(w io.Writer, forest *model.Forest) {
	if len(forest.Roots) == 0 && len(forest.Orphans) == 0 {
		_, _ = fmt.Fprintln(w, "No in-flight operations found.")
		return
	}

	// Group roots by component for section headers.
	groups := make(map[string][]*model.Node)
	var order []string
	for _, root := range forest.Roots {
		comp := root.IFO.Spec.Component
		if comp == "" {
			comp = "(none)"
		}
		if _, seen := groups[comp]; !seen {
			order = append(order, comp)
		}
		groups[comp] = append(groups[comp], root)
	}

	for i, comp := range order {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w, p.Color.Bold(comp))
		roots := groups[comp]
		for _, root := range roots {
			p.printSubtree(w, root, "  ", "  ")
		}
	}

	if len(forest.Orphans) > 0 {
		if len(forest.Roots) > 0 {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w, p.Color.Bold("(ungrouped)"))
		for _, orphan := range forest.Orphans {
			p.printSubtree(w, orphan, "  ", "  ")
		}
	}
}

// PrintTree renders a single tree (for --for mode).
func (p *TreePrinter) PrintTree(w io.Writer, root *model.Node) {
	p.printSubtree(w, root, "", "")
}

func (p *TreePrinter) printSubtree(w io.Writer, n *model.Node, linePrefix, childPrefix string) {
	subject := formatSubject(n)
	operation := colorizeOperation(p.Color, n.IFO.Spec.Operation)
	age := p.Color.Dim(FormatAge(n.IFO.CreationTimestamp.Time))

	_, _ = fmt.Fprintf(w, "%s%-50s %s  %s\n", linePrefix, subject, operation, age)

	for i, child := range n.Children {
		isLast := i == len(n.Children)-1
		var connector, nextPrefix string
		if isLast {
			connector = childPrefix + treeCorner
			nextPrefix = childPrefix + treeBlank
		} else {
			connector = childPrefix + treeBranch
			nextPrefix = childPrefix + treeBar
		}
		p.printSubtree(w, child, connector, nextPrefix)
	}
}

func colorizeOperation(c *ColorWriter, op string) string {
	if isErrorOperation(op) {
		return c.Red(op)
	}
	if isWarningOperation(op) {
		return c.Yellow(op)
	}
	return c.Green(op)
}

func formatSubject(n *model.Node) string {
	s := n.IFO.Spec.Subject
	if s.Namespace != "" {
		ns := s.Namespace
		if len(ns) > 20 {
			ns = ns[:17] + "..."
		}
		return fmt.Sprintf("%s/%s (%s)", s.Kind, s.Name, ns)
	}
	return fmt.Sprintf("%s/%s", s.Kind, s.Name)
}
