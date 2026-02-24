package evaluator

import (
	"sync"

	"github.com/google/cel-go/cel"
)

// ProgramCache caches compiled CEL programs for performance
type ProgramCache struct {
	mu       sync.RWMutex
	programs map[string]cel.Program
	env      *cel.Env
}

// NewProgramCache creates a new program cache with Kubernetes-aware CEL environment
func NewProgramCache() (*ProgramCache, error) {
	// Create CEL environment with Kubernetes object support
	env, err := cel.NewEnv(
		cel.Variable("object", cel.DynType), // Unstructured object
	)
	if err != nil {
		return nil, err
	}

	return &ProgramCache{
		programs: make(map[string]cel.Program),
		env:      env,
	}, nil
}

// GetOrCompile retrieves a cached program or compiles a new one
func (c *ProgramCache) GetOrCompile(expression string) (cel.Program, error) {
	// Try read lock first for cache hit
	c.mu.RLock()
	if prog, ok := c.programs[expression]; ok {
		c.mu.RUnlock()
		return prog, nil
	}
	c.mu.RUnlock()

	// Compile with write lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if prog, ok := c.programs[expression]; ok {
		return prog, nil
	}

	// Compile and type-check expression
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	// Create program from AST
	prog, err := c.env.Program(ast)
	if err != nil {
		return nil, err
	}

	c.programs[expression] = prog
	return prog, nil
}
