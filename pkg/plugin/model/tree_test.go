package model

import (
	"testing"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func makeIFOWithUID(kind, name, namespace, component, uid, operation string) api.InFlightOperation {
	return api.InFlightOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name + "-" + operation,
			Labels:            make(map[string]string),
			CreationTimestamp: metav1.Now(),
		},
		Spec: api.InFlightOperationSpec{
			Operation: operation,
			Component: component,
			Subject: api.SubjectReference{
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
				UID:       uid,
			},
		},
	}
}

func makeIFOWithOwnerRef(kind, name, namespace, component, uid, operation, ownerUID string) api.InFlightOperation {
	ifo := makeIFOWithUID(kind, name, namespace, component, uid, operation)
	ifo.Spec.Subject.OwnerReferences = []metav1.OwnerReference{
		{UID: types.UID(ownerUID)},
	}
	return ifo
}

func TestBuildForest_SiblingOperationsMerged(t *testing.T) {
	// Two operations on the same CSV (same subject UID).
	csv1 := makeIFOWithUID("ClusterServiceVersion", "csv1", "openshift-cnv", "olm", "uid-csv", "Installing")
	csv2 := makeIFOWithUID("ClusterServiceVersion", "csv1", "openshift-cnv", "olm", "uid-csv", "Pending")

	ifos := []api.InFlightOperation{csv1, csv2}
	forest := BuildForest(ifos, nil)

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root (merged), got %d", len(forest.Roots))
	}
	node := forest.Roots[0]
	if len(node.Siblings) != 1 {
		t.Fatalf("expected 1 sibling, got %d", len(node.Siblings))
	}
	if node.IFO.Spec.Operation != "Installing" {
		t.Errorf("expected primary operation Installing, got %s", node.IFO.Spec.Operation)
	}
	if node.Siblings[0].Spec.Operation != "Pending" {
		t.Errorf("expected sibling operation Pending, got %s", node.Siblings[0].Spec.Operation)
	}
}

func TestBuildForest_SiblingWithChildren(t *testing.T) {
	// CSV has two operations; a Deployment is owned by the CSV.
	csv1 := makeIFOWithUID("ClusterServiceVersion", "csv1", "openshift-cnv", "olm", "uid-csv", "Installing")
	csv2 := makeIFOWithUID("ClusterServiceVersion", "csv1", "openshift-cnv", "olm", "uid-csv", "Pending")
	dep := makeIFOWithOwnerRef("Deployment", "hco-operator", "openshift-cnv", "olm", "uid-dep", "Progressing", "uid-csv")

	ifos := []api.InFlightOperation{csv1, csv2, dep}
	forest := BuildForest(ifos, nil)

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(forest.Roots))
	}
	root := forest.Roots[0]
	if root.IFO.Spec.Subject.Kind != "ClusterServiceVersion" {
		t.Errorf("expected CSV as root, got %s", root.IFO.Spec.Subject.Kind)
	}
	if len(root.Siblings) != 1 {
		t.Errorf("expected 1 sibling on CSV, got %d", len(root.Siblings))
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (Deployment), got %d", len(root.Children))
	}
	if root.Children[0].IFO.Spec.Subject.Kind != "Deployment" {
		t.Errorf("expected Deployment as child, got %s", root.Children[0].IFO.Spec.Subject.Kind)
	}
}

func TestBuildForest_NoUIDNoMerge(t *testing.T) {
	// IFOs without UIDs should not be merged even if they have the same name.
	ifo1 := makeIFOWithUID("VirtualMachine", "vm1", "default", "kubevirt", "", "Starting")
	ifo2 := makeIFOWithUID("VirtualMachine", "vm1", "default", "kubevirt", "", "Migrating")

	ifos := []api.InFlightOperation{ifo1, ifo2}
	forest := BuildForest(ifos, nil)

	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots (no UID, no merge), got %d", len(forest.Roots))
	}
}

func TestBuildForest_OwnerRefLinking(t *testing.T) {
	// Basic ownerRef tree: VM owns VMI.
	vm := makeIFOWithUID("VirtualMachine", "my-vm", "default", "kubevirt", "uid-vm", "Starting")
	vmi := makeIFOWithOwnerRef(
		"VirtualMachineInstance", "my-vm-abc", "default", "kubevirt", "uid-vmi", "Scheduling", "uid-vm")

	ifos := []api.InFlightOperation{vm, vmi}
	forest := BuildForest(ifos, nil)

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(forest.Roots))
	}
	if forest.Roots[0].IFO.Spec.Subject.Kind != "VirtualMachine" {
		t.Errorf("expected VM as root, got %s", forest.Roots[0].IFO.Spec.Subject.Kind)
	}
	if len(forest.Roots[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(forest.Roots[0].Children))
	}
	if forest.Roots[0].Children[0].IFO.Spec.Subject.Kind != "VirtualMachineInstance" {
		t.Errorf("expected VMI as child, got %s", forest.Roots[0].Children[0].IFO.Spec.Subject.Kind)
	}
}
