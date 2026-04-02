package watch

import (
	"testing"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func makeTestSubject(name, namespace, kind, apiVersion string, uid types.UID) *api.Subject {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"uid":       string(uid),
			},
		},
	}
}

func makeTestSubjectWithOwner(name, namespace, kind, apiVersion string, uid types.UID, ownerName, ownerKind, ownerAPIVersion string, ownerUID types.UID) *api.Subject {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"uid":       string(uid),
				"ownerReferences": []any{
					map[string]any{
						"apiVersion": ownerAPIVersion,
						"kind":       ownerKind,
						"name":       ownerName,
						"uid":        string(ownerUID),
					},
				},
			},
		},
	}
}

func makeTestRuleSet(name, component string) *api.OperationRuleSet {
	return &api.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: api.OperationRuleSetSpec{
			Component: component,
		},
	}
}

func TestBuild(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "abc-123")
	ruleset := makeTestRuleSet("vm-rules", "kubevirt")

	ifo := ops.Build(subject, "Migrating", ruleset, nil)

	if ifo.Spec.Operation != "Migrating" {
		t.Errorf("expected operation Migrating, got %s", ifo.Spec.Operation)
	}
	if ifo.Spec.RuleSet != "vm-rules" {
		t.Errorf("expected ruleset vm-rules, got %s", ifo.Spec.RuleSet)
	}
	if ifo.Spec.Component != "kubevirt" {
		t.Errorf("expected component kubevirt, got %s", ifo.Spec.Component)
	}
	if ifo.Spec.Subject.Name != "my-vm" {
		t.Errorf("expected subject name my-vm, got %s", ifo.Spec.Subject.Name)
	}
	if ifo.Spec.Subject.Namespace != "default" {
		t.Errorf("expected subject namespace default, got %s", ifo.Spec.Subject.Namespace)
	}
	if ifo.Spec.Subject.Kind != "VirtualMachine" {
		t.Errorf("expected subject kind VirtualMachine, got %s", ifo.Spec.Subject.Kind)
	}
	if ifo.Spec.Subject.APIVersion != "kubevirt.io/v1" {
		t.Errorf("expected subject apiVersion kubevirt.io/v1, got %s", ifo.Spec.Subject.APIVersion)
	}
	if ifo.Spec.Subject.UID != "abc-123" {
		t.Errorf("expected subject UID abc-123, got %s", ifo.Spec.Subject.UID)
	}
	if ifo.GenerateName != "my-vm-" {
		t.Errorf("expected generateName my-vm-, got %s", ifo.GenerateName)
	}
}

func TestSubjectLabels(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "uid-123")

	labels := ops.subjectLabels(subject)

	expected := map[string]string{
		api.LabelSubjectUID:       "uid-123",
		api.LabelSubjectName:      "my-vm",
		api.LabelSubjectNamespace: "default",
		api.LabelSubjectKind:      "VirtualMachine",
	}
	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("label %s: expected %s, got %s", k, v, labels[k])
		}
	}
	if len(labels) != len(expected) {
		t.Errorf("expected %d labels, got %d", len(expected), len(labels))
	}
}

func TestOperationLabels(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "uid-123")
	ruleset := makeTestRuleSet("vm-rules", "kubevirt")

	labels := ops.operationLabels(subject, "Migrating", ruleset, nil)

	if labels[api.LabelOperation] != "Migrating" {
		t.Errorf("expected operation label Migrating, got %s", labels[api.LabelOperation])
	}
	if labels[api.LabelRuleSet] != "vm-rules" {
		t.Errorf("expected ruleset label vm-rules, got %s", labels[api.LabelRuleSet])
	}
	if labels[api.LabelComponent] != "kubevirt" {
		t.Errorf("expected component label kubevirt, got %s", labels[api.LabelComponent])
	}
	// Should also include subject labels
	if labels[api.LabelSubjectName] != "my-vm" {
		t.Errorf("expected subject name label my-vm, got %s", labels[api.LabelSubjectName])
	}
}

func TestOperationLabelsWithDynamicLabels(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "uid-123")
	ruleset := makeTestRuleSet("vm-rules", "kubevirt")
	dynamicLabels := map[string]string{
		"node":  "node-1",
		"phase": "Migrating",
	}

	labels := ops.operationLabels(subject, "Migrating", ruleset, dynamicLabels)

	if labels["node"] != "node-1" {
		t.Errorf("expected node=node-1, got %s", labels["node"])
	}
	if labels["phase"] != "Migrating" {
		t.Errorf("expected phase=Migrating, got %s", labels["phase"])
	}
	// Built-in labels should still be present
	if labels[api.LabelOperation] != "Migrating" {
		t.Errorf("expected operation label Migrating, got %s", labels[api.LabelOperation])
	}
}

func TestOperationLabelsBuiltinOverrideDynamic(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "uid-123")
	ruleset := makeTestRuleSet("vm-rules", "kubevirt")
	// Dynamic labels try to override built-in labels — built-in should win
	dynamicLabels := map[string]string{
		api.LabelSubjectName: "hacked",
		api.LabelOperation:   "hacked",
	}

	labels := ops.operationLabels(subject, "Migrating", ruleset, dynamicLabels)

	if labels[api.LabelSubjectName] != "my-vm" {
		t.Errorf("built-in label should win: expected my-vm, got %s", labels[api.LabelSubjectName])
	}
	if labels[api.LabelOperation] != "Migrating" {
		t.Errorf("built-in label should win: expected Migrating, got %s", labels[api.LabelOperation])
	}
}

func TestOperationLabelsStaticOverrideDynamic(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "uid-123")
	ruleset := &api.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "vm-rules"},
		Spec: api.OperationRuleSetSpec{
			Component: "kubevirt",
			Labels: map[string]string{
				"shared-key": "static-value",
			},
		},
	}
	dynamicLabels := map[string]string{
		"shared-key": "dynamic-value",
	}

	labels := ops.operationLabels(subject, "Migrating", ruleset, dynamicLabels)

	if labels["shared-key"] != "static-value" {
		t.Errorf("static label should win over dynamic: expected static-value, got %s", labels["shared-key"])
	}
}

func TestOperationLabelsWithOwnerRef(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubjectWithOwner(
		"aaq-operator", "openshift-cnv", "Deployment", "apps/v1", "deploy-uid",
		"kubevirt-hyperconverged-operator.v4.20.8", "ClusterServiceVersion", "operators.coreos.com/v1alpha1", "csv-uid",
	)
	ruleset := makeTestRuleSet("deployment-rules", "")

	labels := ops.operationLabels(subject, "Rollout", ruleset, nil)

	if labels[api.LabelOwnerUID] != "csv-uid" {
		t.Errorf("expected owner UID csv-uid, got %s", labels[api.LabelOwnerUID])
	}
	if labels[api.LabelOwnerName] != "kubevirt-hyperconverged-operator.v4.20.8" {
		t.Errorf("expected owner name, got %s", labels[api.LabelOwnerName])
	}
	if labels[api.LabelOwnerKind] != "ClusterServiceVersion" {
		t.Errorf("expected owner kind ClusterServiceVersion, got %s", labels[api.LabelOwnerKind])
	}
	if labels[api.LabelOwnerGroup] != "operators.coreos.com" {
		t.Errorf("expected owner group operators.coreos.com, got %s", labels[api.LabelOwnerGroup])
	}
	if labels[api.LabelOwnerVersion] != "v1alpha1" {
		t.Errorf("expected owner version v1alpha1, got %s", labels[api.LabelOwnerVersion])
	}
}

func TestOperationLabelsOwnerCoreAPIGroup(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubjectWithOwner(
		"my-pod", "default", "Pod", "v1", "pod-uid",
		"my-rs", "ReplicaSet", "apps/v1", "rs-uid",
	)
	ruleset := makeTestRuleSet("pod-rules", "")

	labels := ops.operationLabels(subject, "Starting", ruleset, nil)

	if labels[api.LabelOwnerGroup] != "apps" {
		t.Errorf("expected owner group apps, got %s", labels[api.LabelOwnerGroup])
	}
	if labels[api.LabelOwnerVersion] != "v1" {
		t.Errorf("expected owner version v1, got %s", labels[api.LabelOwnerVersion])
	}
}

func TestOperationLabelsWithStaticLabels(t *testing.T) {
	ops := &Operations{}
	subject := makeTestSubject("my-vm", "default", "VirtualMachine", "kubevirt.io/v1", "uid-123")
	ruleset := &api.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "vm-rules"},
		Spec: api.OperationRuleSetSpec{
			Component: "kubevirt",
			Labels: map[string]string{
				"custom-label":  "custom-value",
				"another-label": "another-value",
			},
		},
	}

	labels := ops.operationLabels(subject, "Migrating", ruleset, nil)

	if labels["custom-label"] != "custom-value" {
		t.Errorf("expected custom-label=custom-value, got %s", labels["custom-label"])
	}
	if labels["another-label"] != "another-value" {
		t.Errorf("expected another-label=another-value, got %s", labels["another-label"])
	}
	// Built-in labels should still be present
	if labels[api.LabelOperation] != "Migrating" {
		t.Errorf("expected operation label Migrating, got %s", labels[api.LabelOperation])
	}
}
