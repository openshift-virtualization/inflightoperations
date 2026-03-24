package evaluator

import (
	"fmt"

	api "github.com/ifo-operator/inflightoperations/api/v1alpha1"
	liberr "github.com/ifo-operator/inflightoperations/lib/error"
)

type RuleSetResult struct {
	RuleSet    *api.OperationRuleSet
	Operations []string
	Labels     map[string]string
}

// Evaluator provides CEL expression evaluation for Kubernetes resources
type Evaluator interface {
	// Compile checks that a CEL expression is syntactically valid without evaluating it.
	Compile(expression string) error

	// Evaluate evaluates a single CEL expression against an unstructured object
	Evaluate(subject *api.Subject, rule *api.Rule) (bool, error)

	// EvaluateLabelExpression evaluates a CEL expression that returns a string value
	EvaluateLabelExpression(subject *api.Subject, expression string) (string, error)

	// EvaluateRuleSet evaluates a RuleSet and returns as RuleSetResult.
	EvaluateRuleSet(subject *api.Subject, ruleset *api.OperationRuleSet) (RuleSetResult, error)
}

// celEvaluator implements the Evaluator interface
type celEvaluator struct {
	programCache *ProgramCache
}

// NewEvaluator creates a new CEL evaluator with program caching
func NewEvaluator() (Evaluator, error) {
	cache, err := NewProgramCache()
	if err != nil {
		return nil, fmt.Errorf("failed to create program cache: %w", err)
	}

	return &celEvaluator{
		programCache: cache,
	}, nil
}

// Compile checks that a CEL expression is syntactically valid without evaluating it.
func (r *celEvaluator) Compile(expression string) error {
	_, err := r.programCache.GetOrCompile(expression)
	return err
}

// Evaluate evaluates a single CEL expression against an unstructured object
func (r *celEvaluator) Evaluate(subject *api.Subject, rule *api.Rule) (value bool, err error) {
	prog, err := r.programCache.GetOrCompile(rule.Expression)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	vars := map[string]interface{}{
		"object": subject.Object,
	}

	out, _, err := prog.Eval(vars)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Convert result to boolean
	value, ok := out.Value().(bool)
	if !ok {
		err = fmt.Errorf("expression did not return a boolean value, got: %v (type: %T)", out.Value(), out.Value())
		return
	}
	return
}

// EvaluateLabelExpression evaluates a CEL expression that returns a string value
func (r *celEvaluator) EvaluateLabelExpression(subject *api.Subject, expression string) (value string, err error) {
	prog, err := r.programCache.GetOrCompile(expression)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	vars := map[string]interface{}{
		"object": subject.Object,
	}

	out, _, err := prog.Eval(vars)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	value, ok := out.Value().(string)
	if !ok {
		err = fmt.Errorf("label expression did not return a string value, got: %v (type: %T)", out.Value(), out.Value())
		return
	}
	return
}

// EvaluateRuleSet evaluates all rules and returns matching operation names
func (r *celEvaluator) EvaluateRuleSet(subject *api.Subject, ruleset *api.OperationRuleSet) (result RuleSetResult, err error) {
	result = RuleSetResult{
		RuleSet: ruleset,
		Labels:  make(map[string]string),
	}
	for _, rule := range ruleset.Rules() {
		var matched bool
		matched, err = r.Evaluate(subject, &rule)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		if matched {
			result.Operations = append(result.Operations, rule.Operation)
		}
	}
	for _, le := range ruleset.Spec.LabelExpressions {
		val, lErr := r.EvaluateLabelExpression(subject, le.Expression)
		if lErr != nil {
			continue
		}
		result.Labels[le.Key] = val
	}
	return
}
