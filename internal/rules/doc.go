// Package rules provides types and utilities for CEL evaluation rules.
//
// Rules can be loaded from YAML files or constructed programmatically.
// The Rule type is used by the evaluator package for expression evaluation.
//
// Example YAML format:
//
//	gvk:
//	  group: kubevirt.io
//	  version: v1
//	  kind: VirtualMachine
//	rules:
//	  - operation: Migrating
//	    expression: object.status.printableStatus == "Migrating"
package rules
