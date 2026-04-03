package model

import (
	"slices"
	"strings"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
)

// BuildForest constructs a forest of IFO trees using ownerReference-based correlation,
// then applies heuristic correlators for non-ownerRef relationships.
func BuildForest(ifos []api.InFlightOperation, correlators []Correlator) *Forest {
	forest := buildOwnerRefTrees(ifos)
	for _, c := range correlators {
		c.Correlate(forest)
	}
	sortForest(forest)
	return forest
}

// buildOwnerRefTrees creates trees by matching subject UIDs to ownerReference UIDs.
// If IFO A's subject UID appears in IFO B's subject.ownerReferences, then A is B's parent.
func buildOwnerRefTrees(ifos []api.InFlightOperation) *Forest {
	nodes := make([]*Node, 0, len(ifos))
	bySubjectUID := make(map[string]*Node, len(ifos))

	// Create nodes and index by subject UID.
	// When multiple IFOs share a subject UID (multiple operations on the
	// same resource), merge them into one node so children link to the
	// subject rather than an arbitrary operation.
	for i := range ifos {
		uid := ifos[i].Spec.Subject.UID
		if uid != "" {
			if existing, ok := bySubjectUID[uid]; ok {
				existing.Siblings = append(existing.Siblings, &ifos[i])
				continue
			}
		}
		n := &Node{IFO: &ifos[i]}
		nodes = append(nodes, n)
		if uid != "" {
			bySubjectUID[uid] = n
		}
	}

	// Link children to parents via ownerReferences.
	for _, n := range nodes {
		for _, ownerRef := range n.IFO.Spec.Subject.OwnerReferences {
			if parent, ok := bySubjectUID[string(ownerRef.UID)]; ok {
				n.Parent = parent
				parent.Children = append(parent.Children, n)
				break // use first matching parent
			}
		}
	}

	// Separate roots from orphans.
	forest := &Forest{}
	for _, n := range nodes {
		if n.Parent == nil {
			forest.Roots = append(forest.Roots, n)
		}
	}

	return forest
}

// sortForest sorts roots by component, then by kind, then by name.
// Children within each node are sorted by kind then name.
func sortForest(forest *Forest) {
	slices.SortFunc(forest.Roots, compareNodes)
	for _, root := range forest.Roots {
		sortChildren(root)
	}
	slices.SortFunc(forest.Orphans, compareNodes)
}

func sortChildren(n *Node) {
	slices.SortFunc(n.Children, compareNodes)
	for _, child := range n.Children {
		sortChildren(child)
	}
}

func compareNodes(a, b *Node) int {
	// Sort by component first.
	if c := strings.Compare(a.IFO.Spec.Component, b.IFO.Spec.Component); c != 0 {
		return c
	}
	// Then by kind.
	if c := strings.Compare(a.IFO.Spec.Subject.Kind, b.IFO.Spec.Subject.Kind); c != 0 {
		return c
	}
	// Then by name.
	return strings.Compare(a.IFO.Spec.Subject.Name, b.IFO.Spec.Subject.Name)
}
