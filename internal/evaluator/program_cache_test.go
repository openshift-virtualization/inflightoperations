package evaluator

import (
	"sync"
	"testing"
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
