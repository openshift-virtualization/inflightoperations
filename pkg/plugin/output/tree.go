package output

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/openshift-virtualization/inflightoperations/pkg/plugin/model"
)

const (
	treeBranch = "├── "
	treeCorner = "└── "
	treeBar    = "│   "
	treeBlank  = "    "

	clusterScoped = "(cluster-scoped)"
)

// TreePrinter renders a forest of IFO nodes as an indented tree.
type TreePrinter struct {
	Color *ColorWriter
}

// PrintForest renders the entire forest grouped by component, then namespace.
func (p *TreePrinter) PrintForest(w io.Writer, forest *model.Forest) {
	if len(forest.Roots) == 0 && len(forest.Orphans) == 0 {
		_, _ = fmt.Fprintln(w, "No in-flight operations found.")
		return
	}

	allRoots := append(forest.Roots, forest.Orphans...)

	// Compute column width across the entire forest.
	colWidth := 0
	for _, root := range allRoots {
		if w := p.computeMaxWidth(root, "    ", "    "); w > colWidth {
			colWidth = w
		}
	}
	colWidth += 2 // minimum gap before operation column

	// Compute max operations width for age column alignment.
	opColWidth := 0
	for _, root := range allRoots {
		if w := computeMaxOpsWidth(root); w > opColWidth {
			opColWidth = w
		}
	}

	// Group roots by component, then by namespace.
	type compGroup struct {
		namespaces map[string][]*model.Node
		nsOrder    []string
	}
	groups := make(map[string]*compGroup)
	var compOrder []string

	for _, root := range allRoots {
		comp := root.IFO.Spec.Component
		if comp == "" {
			comp = "(none)"
		}
		ns := rootNamespace(root)

		cg, ok := groups[comp]
		if !ok {
			cg = &compGroup{namespaces: make(map[string][]*model.Node)}
			groups[comp] = cg
			compOrder = append(compOrder, comp)
		}
		if _, seen := cg.namespaces[ns]; !seen {
			cg.nsOrder = append(cg.nsOrder, ns)
		}
		cg.namespaces[ns] = append(cg.namespaces[ns], root)
	}

	for i, comp := range compOrder {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w, p.Color.Bold(comp))

		cg := groups[comp]
		slices.Sort(cg.nsOrder)
		for _, ns := range cg.nsOrder {
			_, _ = fmt.Fprintf(w, "  %s\n", p.Color.BrightYellow(ns))
			for _, root := range cg.namespaces[ns] {
				p.printSubtree(w, root, "    ", "    ", colWidth, opColWidth)
			}
		}
	}
}

// PrintTree renders a single tree (for --for mode).
func (p *TreePrinter) PrintTree(w io.Writer, root *model.Node) {
	ns := rootNamespace(root)
	_, _ = fmt.Fprintln(w, p.Color.BrightYellow(ns))

	colWidth := p.computeMaxWidth(root, "  ", "  ") + 2
	opColWidth := computeMaxOpsWidth(root)
	p.printSubtree(w, root, "  ", "  ", colWidth, opColWidth)
}

// computeMaxWidth returns the maximum (prefix + subject) visual width across a subtree.
func (p *TreePrinter) computeMaxWidth(n *model.Node, linePrefix, childPrefix string) int {
	maxW := displayWidth(linePrefix) + len(formatSubject(n))
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

func (p *TreePrinter) printSubtree(w io.Writer, n *model.Node, linePrefix, childPrefix string, colWidth, opColWidth int) {
	subject := formatSubject(n)
	operations := p.formatOperations(n)
	plainOpsWidth := operationsWidth(n)
	age := p.formatAge(n)

	padding := colWidth - displayWidth(linePrefix) - len(subject)
	if padding < 2 {
		padding = 2
	}
	opPadding := opColWidth - plainOpsWidth
	if opPadding < 0 {
		opPadding = 0
	}
	_, _ = fmt.Fprintf(w, "%s%s%*s%s%*s  %s\n", linePrefix, subject, padding, "", operations, opPadding, "", age)

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
		p.printSubtree(w, child, connector, nextPrefix, colWidth, opColWidth)
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

// operationsWidth returns the plain-text visual width of the operations
// string for a node (without ANSI color codes).
func operationsWidth(n *model.Node) int {
	ops := []string{n.IFO.Spec.Operation}
	for _, sib := range n.Siblings {
		ops = append(ops, sib.Spec.Operation)
	}
	slices.Sort(ops)
	return len(strings.Join(ops, ", "))
}

// computeMaxOpsWidth returns the maximum operations width across a subtree.
func computeMaxOpsWidth(n *model.Node) int {
	maxW := operationsWidth(n)
	for _, child := range n.Children {
		if w := computeMaxOpsWidth(child); w > maxW {
			maxW = w
		}
	}
	return maxW
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
	return fmt.Sprintf("%s/%s", s.Kind, s.Name)
}

// rootNamespace returns the display namespace for a tree root.
func rootNamespace(n *model.Node) string {
	ns := n.IFO.Spec.Subject.Namespace
	if ns == "" {
		return clusterScoped
	}
	return ns
}

// displayWidth returns the visual width of a string, counting runes
// rather than bytes. Tree-drawing characters like ├, └, │ are multi-byte
// UTF-8 but each occupies one terminal cell.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}
