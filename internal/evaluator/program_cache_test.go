package evaluator

import (
	"sync"
	"testing"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetOrCompileCacheHit(t *testing.T) {
	cache, err := NewProgramCache()
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	expr := "object.status.phase == 'Running'"

	prog1, err := cache.GetOrCompile(expr)
	if err != nil {
		t.Fatalf("first compile failed: %v", err)
	}

	prog2, err := cache.GetOrCompile(expr)
	if err != nil {
		t.Fatalf("second compile failed: %v", err)
	}

	// Same program instance should be returned from cache
	if &prog1 == nil || &prog2 == nil {
		t.Fatal("programs should not be nil")
	}

	// Verify cache has exactly one entry
	cache.mu.RLock()
	count := len(cache.programs)
	cache.mu.RUnlock()
	if count != 1 {
		t.Fatalf("expected 1 cached program, got %d", count)
	}
}

func TestGetOrCompileInvalidExpression(t *testing.T) {
	cache, err := NewProgramCache()
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	_, err = cache.GetOrCompile("this is not valid CEL !!!@#$")
	if err == nil {
		t.Fatal("expected error for invalid CEL expression")
	}

	// Invalid expressions should not be cached
	cache.mu.RLock()
	count := len(cache.programs)
	cache.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected 0 cached programs after error, got %d", count)
	}
}

func TestEvaluateLabelExpressionString(t *testing.T) {
	eval, err := NewEvaluator()
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}
	subject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "my-vm",
			},
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	val, err := eval.EvaluateLabelExpression(subject, "object.status.phase")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "Running" {
		t.Errorf("expected 'Running', got '%s'", val)
	}
}

func TestEvaluateLabelExpressionNonString(t *testing.T) {
	eval, err := NewEvaluator()
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}
	subject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
	}

	_, err = eval.EvaluateLabelExpression(subject, "object.spec.replicas")
	if err == nil {
		t.Fatal("expected error for non-string return")
	}
}

func TestEvaluateLabelExpressionInvalid(t *testing.T) {
	eval, err := NewEvaluator()
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}
	subject := &unstructured.Unstructured{Object: map[string]interface{}{}}

	_, err = eval.EvaluateLabelExpression(subject, "invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestEvaluateRuleSetWithLabelExpressions(t *testing.T) {
	eval, err := NewEvaluator()
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}
	subject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase":    "Migrating",
				"nodeName": "node-1",
			},
		},
	}
	ruleset := &api.OperationRuleSet{
		Spec: api.OperationRuleSetSpec{
			Rules: []api.Rule{
				{Operation: "Migrating", Expression: "object.status.phase == 'Migrating'"},
			},
			LabelExpressions: []api.LabelExpression{
				{Key: "node", Expression: "object.status.nodeName"},
				{Key: "phase", Expression: "object.status.phase"},
			},
		},
	}

	result, err := eval.EvaluateRuleSet(subject, ruleset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Operations) != 1 || result.Operations[0] != "Migrating" {
		t.Errorf("expected [Migrating], got %v", result.Operations)
	}
	if result.Labels["node"] != "node-1" {
		t.Errorf("expected label node=node-1, got %s", result.Labels["node"])
	}
	if result.Labels["phase"] != "Migrating" {
		t.Errorf("expected label phase=Migrating, got %s", result.Labels["phase"])
	}
}

func TestEvaluateRuleSetLabelExpressionErrorSkipped(t *testing.T) {
	eval, err := NewEvaluator()
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}
	subject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}
	ruleset := &api.OperationRuleSet{
		Spec: api.OperationRuleSetSpec{
			Rules: []api.Rule{
				{Operation: "Running", Expression: "object.status.phase == 'Running'"},
			},
			LabelExpressions: []api.LabelExpression{
				{Key: "phase", Expression: "object.status.phase"},
				{Key: "missing", Expression: "object.status.nonexistent"},
			},
		},
	}

	result, err := eval.EvaluateRuleSet(subject, ruleset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Labels["phase"] != "Running" {
		t.Errorf("expected label phase=Running, got %s", result.Labels["phase"])
	}
	if _, ok := result.Labels["missing"]; ok {
		t.Error("expected missing label to be absent due to evaluation error")
	}
}

func TestGetOrCompileConcurrency(t *testing.T) {
	cache, err := NewProgramCache()
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	expressions := []string{
		"object.status.phase == 'Running'",
		"has(object.status.conditions)",
		"object.metadata.name == 'test'",
		"object.spec.replicas > 1",
		"has(object.status) && object.status.ready == true",
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(expressions)*10)

	for i := 0; i < 10; i++ {
		for _, expr := range expressions {
			wg.Add(1)
			go func(e string) {
				defer wg.Done()
				_, err := cache.GetOrCompile(e)
				if err != nil {
					errors <- err
				}
			}(expr)
		}
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent compile error: %v", err)
	}

	cache.mu.RLock()
	count := len(cache.programs)
	cache.mu.RUnlock()
	if count != len(expressions) {
		t.Fatalf("expected %d cached programs, got %d", len(expressions), count)
	}
}
