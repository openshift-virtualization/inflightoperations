package model

// OLMCorrelator groups OLM resources (Subscription, InstallPlan, CSV) in the same namespace.
// Subscription becomes the root; InstallPlan and CSV become children.
type OLMCorrelator struct{}

func (c *OLMCorrelator) Name() string { return "olm" }

func (c *OLMCorrelator) Correlate(forest *Forest) {
	// Index OLM roots by namespace.
	type olmGroup struct {
		subscription *Node
		children     []*Node
	}
	groups := make(map[string]*olmGroup)

	remaining := make([]*Node, 0, len(forest.Roots))
	for _, root := range forest.Roots {
		if root.IFO.Spec.Component != "olm" {
			remaining = append(remaining, root)
			continue
		}
		ns := root.IFO.Spec.Subject.Namespace
		g, ok := groups[ns]
		if !ok {
			g = &olmGroup{}
			groups[ns] = g
		}
		if root.IFO.Spec.Subject.Kind == "Subscription" {
			g.subscription = root
		} else {
			g.children = append(g.children, root)
		}
	}

	for _, g := range groups {
		if g.subscription == nil {
			// No subscription root — leave children as independent roots.
			remaining = append(remaining, g.children...)
			continue
		}
		for _, child := range g.children {
			child.Parent = g.subscription
			g.subscription.Children = append(g.subscription.Children, child)
		}
		remaining = append(remaining, g.subscription)
	}
	forest.Roots = remaining
}

// HCOCorrelator groups HCO-managed component operator CRs under the HCO IFO.
type HCOCorrelator struct{}

func (c *HCOCorrelator) Name() string { return "hco" }

var hcoManagedKinds = map[string]bool{
	"KubeVirt":            true,
	"CDI":                 true,
	"NetworkAddonsConfig": true,
	"SSP":                 true,
	"HostPathProvisioner": true,
	"AAQ":                 true,
}

func (c *HCOCorrelator) Correlate(forest *Forest) {
	// Find HCO root.
	var hcoNode *Node
	for _, root := range forest.Roots {
		if root.IFO.Spec.Subject.Kind == "HyperConverged" {
			hcoNode = root
			break
		}
	}
	if hcoNode == nil {
		return
	}

	var remaining []*Node
	for _, root := range forest.Roots {
		if root == hcoNode {
			remaining = append(remaining, root)
			continue
		}
		if hcoManagedKinds[root.IFO.Spec.Subject.Kind] {
			root.Parent = hcoNode
			hcoNode.Children = append(hcoNode.Children, root)
		} else {
			remaining = append(remaining, root)
		}
	}
	forest.Roots = remaining
}

// ClusterVersionCorrelator groups ClusterOperator IFOs under the ClusterVersion IFO.
type ClusterVersionCorrelator struct{}

func (c *ClusterVersionCorrelator) Name() string { return "clusterversion" }

func (c *ClusterVersionCorrelator) Correlate(forest *Forest) {
	var cvNode *Node
	for _, root := range forest.Roots {
		if root.IFO.Spec.Subject.Kind == "ClusterVersion" {
			cvNode = root
			break
		}
	}
	if cvNode == nil {
		return
	}

	var remaining []*Node
	for _, root := range forest.Roots {
		if root == cvNode {
			remaining = append(remaining, root)
			continue
		}
		if root.IFO.Spec.Subject.Kind == "ClusterOperator" {
			root.Parent = cvNode
			cvNode.Children = append(cvNode.Children, root)
		} else {
			remaining = append(remaining, root)
		}
	}
	forest.Roots = remaining
}

// DefaultCorrelators returns the standard set of heuristic correlators.
func DefaultCorrelators() []Correlator {
	return []Correlator{
		&OLMCorrelator{},
		&HCOCorrelator{},
		&ClusterVersionCorrelator{},
	}
}
