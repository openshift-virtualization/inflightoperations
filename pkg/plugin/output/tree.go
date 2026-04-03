package output

import (
	"fmt"
	"io"
	"slices"
	"strings"

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

	// Compute column width across the entire forest.
	colWidth := 0
	for _, root := range forest.Roots {
		if w := p.computeMaxWidth(root, "  ", "  "); w > colWidth {
			colWidth = w
		}
	}
	for _, orphan := range forest.Orphans {
		if w := p.computeMaxWidth(orphan, "  ", "  "); w > colWidth {
			colWidth = w
		}
	}
	colWidth += 2 // minimum gap before operation column

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
			p.printSubtree(w, root, "  ", "  ", colWidth)
		}
	}

	if len(forest.Orphans) > 0 {
		if len(forest.Roots) > 0 {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w, p.Color.Bold("(ungrouped)"))
		for _, orphan := range forest.Orphans {
			p.printSubtree(w, orphan, "  ", "  ", colWidth)
		}
	}
}

// PrintTree renders a single tree (for --for mode).
func (p *TreePrinter) PrintTree(w io.Writer, root *model.Node) {
	colWidth := p.computeMaxWidth(root, "", "") + 2
	p.printSubtree(w, root, "", "", colWidth)
}

// computeMaxWidth returns the maximum (prefix + subject) width across a subtree.
func (p *TreePrinter) computeMaxWidth(n *model.Node, linePrefix, childPrefix string) int {
	maxW := len(linePrefix) + len(formatSubject(n))
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
		if w := p.computeMaxWidth(child, connector, nextPrefix); w > maxW {
			maxW = w
		}
	}
	return maxW
}

func (p *TreePrinter) printSubtree(w io.Writer, n *model.Node, linePrefix, childPrefix string, colWidth int) {
	subject := formatSubject(n)
	operations := p.formatOperations(n)
	age := p.formatAge(n)

	padding := colWidth - len(linePrefix) - len(subject)
	if padding < 2 {
		padding = 2
	}
	_, _ = fmt.Fprintf(w, "%s%s%*s%s  %s\n", linePrefix, subject, padding, "", operations, age)

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
		p.printSubtree(w, child, connector, nextPrefix, colWidth)
	}
}

// formatOperations returns colorized operation names for a node,
// including any sibling operations on the same subject.
func (p *TreePrinter) formatOperations(n *model.Node) string {
	ops := []string{n.IFO.Spec.Operation}
	for _, sib := range n.Siblings {
		ops = append(ops, sib.Spec.Operation)
	}
	slices.Sort(ops)
	colored := make([]string, len(ops))
	for i, op := range ops {
		colored[i] = colorizeOperation(p.Color, op)
	}
	return strings.Join(colored, ", ")
}

// formatAge returns the age string using the oldest creation timestamp
// across the primary IFO and any siblings.
func (p *TreePrinter) formatAge(n *model.Node) string {
	oldest := n.IFO.CreationTimestamp.Time
	for _, sib := range n.Siblings {
		if sib.CreationTimestamp.Time.Before(oldest) {
			oldest = sib.CreationTimestamp.Time
		}
	}
	return p.Color.Dim(FormatAge(oldest))
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
