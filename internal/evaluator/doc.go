// Package evaluator provides CEL expression evaluation for Kubernetes resources.
//
// The evaluator is thread-safe and uses internal program caching for performance.
// CEL programs are compiled once and cached, so expression evaluation is fast
// (nanoseconds vs milliseconds for compilation).
//
// Example usage:
//
//	eval, err := evaluator.NewEvaluator()
//	if err != nil {
//	    return err
//	}
//
//	operations, err := eval.EvaluateRules(object, rules)
package evaluator
