package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAppliesTo(t *testing.T) {
	makeSubjectInNS := func(ns string) *Subject {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": ns,
				},
			},
		}
	}

	t.Run("empty namespaces matches nothing (current behavior)", func(t *testing.T) {
		ors := &OperationRuleSet{}
		subject := makeSubjectInNS("default")
		// With empty Namespaces, AppliesTo returns false.
		// This is a known bug (should return true per the spec comment).
		if ors.AppliesTo(subject) {
			t.Fatal("expected false with empty namespaces (current behavior)")
		}
	})
	t.Run("matching namespace", func(t *testing.T) {
		ors := &OperationRuleSet{
			Spec: OperationRuleSetSpec{
				Namespaces: []string{"default", "test"},
			},
		}
		subject := makeSubjectInNS("default")
		if !ors.AppliesTo(subject) {
			t.Fatal("expected true for matching namespace")
		}
	})
	t.Run("non-matching namespace", func(t *testing.T) {
		ors := &OperationRuleSet{
			Spec: OperationRuleSetSpec{
				Namespaces: []string{"prod"},
			},
		}
		subject := makeSubjectInNS("default")
		if ors.AppliesTo(subject) {
			t.Fatal("expected false for non-matching namespace")
		}
	})
}
