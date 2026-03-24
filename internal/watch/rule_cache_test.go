/*
Generated-by: Claude Code
*/
package watch

import (
	"fmt"
	"sync"
	"testing"

	"github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRuleCacheAddAndGet(t *testing.T) {
	cache := NewRuleCache()

	rule := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rule"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Migrating", Expression: "true"},
			},
		},
	}

	cache.AddOrUpdateRule(&rule)

	gvr := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}

	rules := cache.List(gvr)
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "test-rule" {
		t.Errorf("Expected rule name 'test-rule', got '%s'", rules[0].Name)
	}
}

func TestRuleCacheUpdate(t *testing.T) {
	cache := NewRuleCache()

	// Add initial rule
	rule := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rule"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Migrating", Expression: "true"},
			},
		},
	}
	cache.AddOrUpdateRule(&rule)

	// Update with new version
	updatedRule := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rule"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Starting", Expression: "true"},
			},
		},
	}
	cache.AddOrUpdateRule(&updatedRule)

	gvr := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}

	rules := cache.List(gvr)
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule after update, got %d", len(rules))
	}
	if len(rules[0].Spec.Rules) != 1 || rules[0].Spec.Rules[0].Operation != "Starting" {
		t.Errorf("Rule was not updated correctly")
	}
}

func TestRuleCacheRemove(t *testing.T) {
	cache := NewRuleCache()

	rule := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rule"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Migrating", Expression: "true"},
			},
		},
	}
	cache.AddOrUpdateRule(&rule)

	gvr := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}

	cache.RemoveRule(&rule)

	rules := cache.List(gvr)
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules after removal, got %d", len(rules))
	}
}

func TestRuleCacheMultipleRulesPerGVR(t *testing.T) {
	cache := NewRuleCache()

	rule1 := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rule-1"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Migrating", Expression: "true"},
			},
		},
	}

	rule2 := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rule-2"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Starting", Expression: "true"},
			},
		},
	}

	cache.AddOrUpdateRule(&rule1)
	cache.AddOrUpdateRule(&rule2)

	gvr := schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}

	rules := cache.List(gvr)
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}
}

func TestRuleCacheConcurrency(t *testing.T) {
	cache := NewRuleCache()
	var wg sync.WaitGroup

	// Simulate concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rule := v1alpha1.OperationRuleSet{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("rule-%d", id)},
				Spec: v1alpha1.OperationRuleSetSpec{
					Target: v1alpha1.GroupVersionResource{
						Group:    "test.io",
						Version:  "v1",
						Resource: "testresources",
					},
					Rules: []v1alpha1.Rule{
						{Operation: "TestOp", Expression: "true"},
					},
				},
			}
			cache.AddOrUpdateRule(&rule)
		}(i)
	}

	// Simulate concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gvr := schema.GroupVersionResource{
				Group:    "test.io",
				Version:  "v1",
				Resource: "testresources",
			}
			_ = cache.List(gvr)
		}()
	}

	wg.Wait()

	// Verify final state
	gvr := schema.GroupVersionResource{
		Group:    "test.io",
		Version:  "v1",
		Resource: "testresources",
	}
	rules := cache.List(gvr)
	if len(rules) != 10 {
		t.Errorf("Expected 10 rules after concurrent operations, got %d", len(rules))
	}
}

func TestRuleCacheGetAllGVRs(t *testing.T) {
	cache := NewRuleCache()

	rule1 := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rule-1"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "kubevirt.io",
				Version:  "v1",
				Resource: "virtualmachines",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Migrating", Expression: "true"},
			},
		},
	}

	rule2 := v1alpha1.OperationRuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rule-2"},
		Spec: v1alpha1.OperationRuleSetSpec{
			Target: v1alpha1.GroupVersionResource{
				Group:    "cdi.kubevirt.io",
				Version:  "v1beta1",
				Resource: "datavolumes",
			},
			Rules: []v1alpha1.Rule{
				{Operation: "Provisioning", Expression: "true"},
			},
		},
	}

	cache.AddOrUpdateRule(&rule1)
	cache.AddOrUpdateRule(&rule2)

	gvrs := cache.GVRs()
	if len(gvrs) != 2 {
		t.Errorf("Expected 2 GVRs, got %d", len(gvrs))
	}
}
