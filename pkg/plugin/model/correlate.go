package model

import (
	"slices"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
)

// LabelCorrelator groups IFOs using ifo.kubevirt.io/correlation-group and
// ifo.kubevirt.io/correlation-role labels. IFOs sharing the same
// correlation-group value (within the same namespace for namespace-scoped
// subjects, or globally for cluster-scoped subjects) are linked into a tree,
// with the node carrying correlation-role=root as the parent.
//
// If no node in a group has correlation-role=root, the group is skipped
// and all members remain independent roots.
type LabelCorrelator struct{}

func (c *LabelCorrelator) Name() string { return "label" }

type groupKey struct {
	group     string
	namespace string
}

func (c *LabelCorrelator) Correlate(forest *Forest) {
	groups := make(map[groupKey][]*Node)
	remaining := make([]*Node, 0, len(forest.Roots))

	for _, root := range forest.Roots {
		group := root.IFO.Labels[api.LabelCorrelationGroup]
		if group == "" {
			remaining = append(remaining, root)
			continue
		}
		key := groupKey{
			group:     group,
			namespace: root.IFO.Spec.Subject.Namespace,
		}
		groups[key] = append(groups[key], root)
	}

	for _, nodes := range groups {
		rootNode := findGroupRoot(nodes)
		if rootNode == nil {
			// No root in this group — leave all as independent roots.
			remaining = append(remaining, nodes...)
			continue
		}
		for _, n := range nodes {
			if n == rootNode {
				continue
			}
			n.Parent = rootNode
			rootNode.Children = append(rootNode.Children, n)
		}
		remaining = append(remaining, rootNode)
	}

	forest.Roots = remaining
}

// findGroupRoot returns the node with correlation-role=root.
// If multiple roots exist, returns the one with the earliest creation timestamp.
// If no root exists, returns nil.
func findGroupRoot(nodes []*Node) *Node {
	var candidates []*Node
	for _, n := range nodes {
		if n.IFO.Labels[api.LabelCorrelationRole] == api.CorrelationRoleRoot {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	// Multiple roots — pick earliest by creation time.
	slices.SortFunc(candidates, func(a, b *Node) int {
		return a.IFO.CreationTimestamp.Compare(b.IFO.CreationTimestamp.Time)
	})
	return candidates[0]
}

// DefaultCorrelators returns the standard set of correlators.
func DefaultCorrelators() []Correlator {
	return []Correlator{
		&LabelCorrelator{},
	}
}
