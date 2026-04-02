package model

import (
	"testing"
	"time"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeIFO(kind, name, namespace, component string, labels map[string]string) api.InFlightOperation {
	if labels == nil {
		labels = make(map[string]string)
	}
	return api.InFlightOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Labels:            labels,
			CreationTimestamp: metav1.Now(),
		},
		Spec: api.InFlightOperationSpec{
			Operation: "Reconciling",
			Component: component,
			Subject: api.SubjectReference{
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

func makeIFOWithTime(
	kind, name, namespace, component string, labels map[string]string, created time.Time,
) api.InFlightOperation {
	ifo := makeIFO(kind, name, namespace, component, labels)
	ifo.CreationTimestamp = metav1.NewTime(created)
	return ifo
}

// assertSingleRootWithChildren verifies that the forest has exactly one root
// of the given kind with the expected number of children.
func assertSingleRootWithChildren(t *testing.T, forest *Forest, rootKind string, childCount int) {
	t.Helper()
	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(forest.Roots))
	}
	if forest.Roots[0].IFO.Spec.Subject.Kind != rootKind {
		t.Errorf("expected root to be %s, got %s", rootKind, forest.Roots[0].IFO.Spec.Subject.Kind)
	}
	if len(forest.Roots[0].Children) != childCount {
		t.Errorf("expected %d children, got %d", childCount, len(forest.Roots[0].Children))
	}
}

func TestLabelCorrelator_BasicGrouping(t *testing.T) {
	root := makeIFO("HyperConverged", "hco", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	child1 := makeIFO("KubeVirt", "kubevirt", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	child2 := makeIFO("CDI", "cdi", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &root},
			{IFO: &child1},
			{IFO: &child2},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	assertSingleRootWithChildren(t, forest, "HyperConverged", 2)
}

func TestLabelCorrelator_NoLabels(t *testing.T) {
	ifo1 := makeIFO("VirtualMachine", "vm1", "default", "kubevirt", nil)
	ifo2 := makeIFO("DataVolume", "dv1", "default", "kubevirt", nil)

	forest := &Forest{
		Roots: []*Node{
			{IFO: &ifo1},
			{IFO: &ifo2},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 2 {
		t.Errorf("expected 2 roots (unchanged), got %d", len(forest.Roots))
	}
}

func TestLabelCorrelator_MixedLabeledAndUnlabeled(t *testing.T) {
	root := makeIFO("HyperConverged", "hco", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	child := makeIFO("KubeVirt", "kubevirt", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	unrelated := makeIFO("VirtualMachine", "my-vm", "default", "kubevirt", nil)

	forest := &Forest{
		Roots: []*Node{
			{IFO: &root},
			{IFO: &child},
			{IFO: &unrelated},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots (1 grouped + 1 unrelated), got %d", len(forest.Roots))
	}

	// Find the grouped root and unrelated root.
	var groupedRoot, unrelatedRoot *Node
	for _, r := range forest.Roots {
		if r.IFO.Spec.Subject.Kind == "HyperConverged" {
			groupedRoot = r
		} else {
			unrelatedRoot = r
		}
	}

	if groupedRoot == nil {
		t.Fatal("expected HyperConverged as grouped root")
	}
	if len(groupedRoot.Children) != 1 {
		t.Errorf("expected 1 child under HCO, got %d", len(groupedRoot.Children))
	}
	if unrelatedRoot == nil || unrelatedRoot.IFO.Spec.Subject.Kind != "VirtualMachine" {
		t.Error("expected VirtualMachine as independent root")
	}
	if len(unrelatedRoot.Children) != 0 {
		t.Errorf("expected VM to have no children, got %d", len(unrelatedRoot.Children))
	}
}

func TestLabelCorrelator_MultipleGroups(t *testing.T) {
	hcoRoot := makeIFO("HyperConverged", "hco", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	kvChild := makeIFO("KubeVirt", "kubevirt", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	cvRoot := makeIFO("ClusterVersion", "version", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	coChild := makeIFO("ClusterOperator", "dns", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &hcoRoot},
			{IFO: &kvChild},
			{IFO: &cvRoot},
			{IFO: &coChild},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots (one per group), got %d", len(forest.Roots))
	}

	for _, root := range forest.Roots {
		if len(root.Children) != 1 {
			t.Errorf("root %s: expected 1 child, got %d", root.IFO.Spec.Subject.Kind, len(root.Children))
		}
	}
}

func TestLabelCorrelator_NamespaceIsolation(t *testing.T) {
	sub1 := makeIFO("Subscription", "sub1", "ns-a", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	csv1 := makeIFO("ClusterServiceVersion", "csv1", "ns-a", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	sub2 := makeIFO("Subscription", "sub2", "ns-b", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	csv2 := makeIFO("ClusterServiceVersion", "csv2", "ns-b", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &sub1},
			{IFO: &csv1},
			{IFO: &sub2},
			{IFO: &csv2},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots (one per namespace), got %d", len(forest.Roots))
	}

	for _, root := range forest.Roots {
		if root.IFO.Spec.Subject.Kind != "Subscription" {
			t.Errorf("expected Subscription as root, got %s", root.IFO.Spec.Subject.Kind)
		}
		if len(root.Children) != 1 {
			t.Errorf("root %s: expected 1 child, got %d", root.IFO.Spec.Subject.Name, len(root.Children))
		}
		// Verify child is in the same namespace as root.
		if root.Children[0].IFO.Spec.Subject.Namespace != root.IFO.Spec.Subject.Namespace {
			t.Errorf("child namespace %s does not match root namespace %s",
				root.Children[0].IFO.Spec.Subject.Namespace, root.IFO.Spec.Subject.Namespace)
		}
	}
}

func TestLabelCorrelator_ClusterScoped(t *testing.T) {
	cvRoot := makeIFO("ClusterVersion", "version", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	co1 := makeIFO("ClusterOperator", "dns", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	co2 := makeIFO("ClusterOperator", "ingress", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &cvRoot},
			{IFO: &co1},
			{IFO: &co2},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	assertSingleRootWithChildren(t, forest, "ClusterVersion", 2)
}

func TestLabelCorrelator_NoRootInGroup(t *testing.T) {
	child1 := makeIFO("KubeVirt", "kubevirt", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	child2 := makeIFO("CDI", "cdi", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &child1},
			{IFO: &child2},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	// No root found — all remain as independent roots.
	if len(forest.Roots) != 2 {
		t.Errorf("expected 2 roots (no grouping without root), got %d", len(forest.Roots))
	}
	for _, root := range forest.Roots {
		if len(root.Children) != 0 {
			t.Errorf("expected no children when no root in group, got %d", len(root.Children))
		}
	}
}

func TestLabelCorrelator_MultipleRoots(t *testing.T) {
	now := time.Now()
	earlier := makeIFOWithTime("Subscription", "sub-old", "default", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	}, now.Add(-5*time.Minute))
	later := makeIFOWithTime("Subscription", "sub-new", "default", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	}, now)
	child := makeIFO("ClusterServiceVersion", "csv1", "default", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &later},
			{IFO: &earlier},
			{IFO: &child},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(forest.Roots))
	}
	if forest.Roots[0].IFO.Spec.Subject.Name != "sub-old" {
		t.Errorf("expected earliest Subscription (sub-old) as root, got %s", forest.Roots[0].IFO.Spec.Subject.Name)
	}
	// The other Subscription and the CSV should be children.
	if len(forest.Roots[0].Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(forest.Roots[0].Children))
	}
}

func TestLabelCorrelator_AlreadyLinkedByOwnerRef(t *testing.T) {
	// Simulate: InstallPlan was already linked under Subscription by ownerRef tree builder.
	// Only Subscription and CSV are in Roots. InstallPlan is NOT in Roots.
	sub := makeIFO("Subscription", "sub1", "default", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	csv := makeIFO("ClusterServiceVersion", "csv1", "default", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	installPlan := makeIFO("InstallPlan", "ip1", "default", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	subNode := &Node{IFO: &sub}
	ipNode := &Node{IFO: &installPlan, Parent: subNode}
	subNode.Children = []*Node{ipNode}
	csvNode := &Node{IFO: &csv}

	// Only sub and csv are in Roots (installPlan is already a child).
	forest := &Forest{
		Roots: []*Node{subNode, csvNode},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(forest.Roots))
	}
	if forest.Roots[0].IFO.Spec.Subject.Kind != "Subscription" {
		t.Errorf("expected Subscription as root, got %s", forest.Roots[0].IFO.Spec.Subject.Kind)
	}
	// Subscription should have 2 children: InstallPlan (from ownerRef) + CSV (from label correlation).
	if len(forest.Roots[0].Children) != 2 {
		t.Errorf("expected 2 children (IP from ownerRef + CSV from labels), got %d", len(forest.Roots[0].Children))
	}
}

func TestLabelCorrelator_HCOScenario(t *testing.T) {
	// HCO stack with correlation labels.
	hco := makeIFO("HyperConverged", "hco", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	kv := makeIFO("KubeVirt", "kubevirt", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	cdi := makeIFO("CDI", "cdi", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	ssp := makeIFO("SSP", "ssp", "openshift-cnv", "kubevirt", map[string]string{
		api.LabelCorrelationGroup: "hco-stack",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	// Workload IFOs — NO correlation labels.
	vm := makeIFO("VirtualMachine", "my-vm", "default", "kubevirt", nil)
	vmi := makeIFO("VirtualMachineInstance", "my-vm-abc", "default", "kubevirt", nil)

	forest := &Forest{
		Roots: []*Node{
			{IFO: &hco},
			{IFO: &kv},
			{IFO: &cdi},
			{IFO: &ssp},
			{IFO: &vm},
			{IFO: &vmi},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	// Should be 3 roots: HCO (with 3 children), VM, VMI.
	if len(forest.Roots) != 3 {
		t.Fatalf("expected 3 roots, got %d", len(forest.Roots))
	}

	var hcoRoot *Node
	for _, r := range forest.Roots {
		if r.IFO.Spec.Subject.Kind == "HyperConverged" {
			hcoRoot = r
		}
	}
	if hcoRoot == nil {
		t.Fatal("expected HyperConverged root")
	}
	if len(hcoRoot.Children) != 3 {
		t.Errorf("expected 3 children under HCO, got %d", len(hcoRoot.Children))
	}
}

func TestLabelCorrelator_OLMScenario(t *testing.T) {
	sub := makeIFO("Subscription", "sub1", "openshift-operators", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	csv := makeIFO("ClusterServiceVersion", "my-operator.v1.0", "openshift-operators", "olm", map[string]string{
		api.LabelCorrelationGroup: "olm-install",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})

	forest := &Forest{
		Roots: []*Node{
			{IFO: &sub},
			{IFO: &csv},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(forest.Roots))
	}
	if forest.Roots[0].IFO.Spec.Subject.Kind != "Subscription" {
		t.Errorf("expected Subscription as root, got %s", forest.Roots[0].IFO.Spec.Subject.Kind)
	}
	if len(forest.Roots[0].Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(forest.Roots[0].Children))
	}
	if forest.Roots[0].Children[0].IFO.Spec.Subject.Kind != "ClusterServiceVersion" {
		t.Errorf("expected CSV as child, got %s", forest.Roots[0].Children[0].IFO.Spec.Subject.Kind)
	}
}

func TestLabelCorrelator_ClusterVersionScenario(t *testing.T) {
	cv := makeIFO("ClusterVersion", "version", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleRoot,
	})
	co1 := makeIFO("ClusterOperator", "dns", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	co2 := makeIFO("ClusterOperator", "ingress", "", "openshift", map[string]string{
		api.LabelCorrelationGroup: "cluster-update",
		api.LabelCorrelationRole:  api.CorrelationRoleChild,
	})
	// MachineConfigPool — same component but NO correlation labels.
	mcp := makeIFO("MachineConfigPool", "worker", "", "openshift", nil)

	forest := &Forest{
		Roots: []*Node{
			{IFO: &cv},
			{IFO: &co1},
			{IFO: &co2},
			{IFO: &mcp},
		},
	}

	c := &LabelCorrelator{}
	c.Correlate(forest)

	// Should be 2 roots: ClusterVersion (with 2 children) + MachineConfigPool.
	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(forest.Roots))
	}

	var cvRoot, mcpRoot *Node
	for _, r := range forest.Roots {
		switch r.IFO.Spec.Subject.Kind {
		case "ClusterVersion":
			cvRoot = r
		case "MachineConfigPool":
			mcpRoot = r
		}
	}
	if cvRoot == nil {
		t.Fatal("expected ClusterVersion root")
	}
	if len(cvRoot.Children) != 2 {
		t.Errorf("expected 2 children under ClusterVersion, got %d", len(cvRoot.Children))
	}
	if mcpRoot == nil {
		t.Fatal("expected MachineConfigPool as independent root")
	}
	if len(mcpRoot.Children) != 0 {
		t.Errorf("expected MCP to have no children, got %d", len(mcpRoot.Children))
	}
}
