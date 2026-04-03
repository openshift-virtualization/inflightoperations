package model

import (
	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
)

// Node wraps an IFO with tree relationship pointers.
// When multiple IFOs share the same subject UID (multiple operations on the
// same resource), the first is stored in IFO and the rest in Siblings.
type Node struct {
	IFO      *api.InFlightOperation
	Siblings []*api.InFlightOperation
	Children []*Node
	Parent   *Node
}

// Forest holds the result of tree construction.
type Forest struct {
	Roots   []*Node
	Orphans []*Node
}

// AllNodes returns all nodes in the forest (roots, their descendants, and orphans).
func (f *Forest) AllNodes() []*Node {
	var all []*Node
	for _, root := range f.Roots {
		all = append(all, collectNodes(root)...)
	}
	all = append(all, f.Orphans...)
	return all
}

func collectNodes(n *Node) []*Node {
	nodes := []*Node{n}
	for _, child := range n.Children {
		nodes = append(nodes, collectNodes(child)...)
	}
	return nodes
}

// Correlator defines a heuristic for grouping IFOs that lack ownerReference relationships.
type Correlator interface {
	Name() string
	Correlate(forest *Forest)
}
