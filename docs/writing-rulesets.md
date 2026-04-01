# Writing Rulesets

An `OperationRuleSet` declares CEL expressions that detect operations on any Kubernetes resource type. When a resource matches a rule, the operator creates an `InFlightOperation` to track it. See the [README](../README.md) for the full CRD reference.

## Anatomy of a RuleSet

```yaml
apiVersion: ifo.kubevirt.io/v1alpha1
kind: OperationRuleSet
metadata:
  name: my-rules
spec:
  # Optional. Identifies the owning component (e.g., "kubevirt", "olm").
  component: my-component

  # Required. The Kubernetes resource type to watch.
  target:
    group: apps          # API group (empty string for core resources like pods)
    version: v1          # API version
    resource: deployments # Plural resource name

  # Required. At least one rule.
  rules:
    - operation: Rollout  # Name of the operation to detect
      expression: |       # CEL expression returning bool
        has(object.status) &&
        has(object.status.readyReplicas) &&
        object.status.readyReplicas < object.spec.replicas

  # Optional. Restrict to specific namespaces. Empty means all namespaces.
  namespaces:
    - production
    - staging

  # Optional. Static labels applied to all created InFlightOperations.
  labels:
    team: platform

  # Optional. Dynamic labels computed from the resource via CEL.
  labelExpressions:
    - key: app-name
      expression: object.metadata.labels.app
```

## CEL Expressions

Rule expressions are evaluated against the watched resource. The resource is available as `object`, an unstructured representation of the full Kubernetes resource.

### Return types

- `spec.rules[].expression` must return `bool`
- `spec.labelExpressions[].expression` must return `string`

### Field access

Use `has()` to check for field existence before accessing nested fields. Accessing a field that doesn't exist causes an evaluation error.

```cel
has(object.status) && has(object.status.phase) && object.status.phase == "Running"
```

### Operators

| Operator | Description |
|----------|-------------|
| `==`, `!=` | Equality |
| `<`, `<=`, `>`, `>=` | Comparison |
| `&&`, `\|\|`, `!` | Logical AND, OR, NOT |
| `? :` | Ternary conditional |

### Functions

| Function | Description |
|----------|-------------|
| `has(field)` | Returns true if the field exists |
| `exists(list, predicate)` | Returns true if any element matches the predicate |
| `size()` | Returns the length of a list or string |
| `startsWith(prefix)` | String prefix check |
| `endsWith(suffix)` | String suffix check |
| `contains(substring)` | String substring check |
| `matches(regex)` | Regular expression match |

### Common patterns

**Simple status field check:**

```cel
has(object.status) &&
has(object.status.printableStatus) &&
object.status.printableStatus == "Migrating"
```

**Searching a conditions array:**

```cel
has(object.status) &&
has(object.status.conditions) &&
object.status.conditions.exists(c, c.type == "Available" && c.status == "False")
```

**Combining multiple conditions:**

```cel
has(object.status) &&
has(object.status.conditions) &&
object.status.conditions.exists(c, c.type == "Available" && c.status == "False") &&
object.status.conditions.exists(c, c.type == "Progressing" && c.status == "True") &&
object.status.conditions.exists(c, c.type == "Degraded" && c.status == "False")
```

**Numeric comparison:**

```cel
has(object.status) &&
(!has(object.status.updatedReplicas) || object.status.updatedReplicas < object.spec.replicas)
```

**Checking for condition existence (without checking status):**

```cel
has(object.status) &&
has(object.status.conditions) &&
object.status.conditions.exists(c, c.type == "BundleUnpacking")
```

## Namespace Scoping

- **`spec.namespaces` omitted or empty**: The ruleset watches all namespaces.
- **`spec.namespaces` set**: Only resources in the listed namespaces are evaluated.

The informer still watches all namespaces at the API level; namespace filtering is applied during expression evaluation.

## Labels

InFlightOperations receive labels from three sources, merged in priority order:

1. **Dynamic labels** (lowest priority) — from `spec.labelExpressions`, evaluated at detection time. If an expression errors, that label is silently skipped.
2. **Static labels** (middle) — from `spec.labels`, always applied.
3. **Built-in labels** (highest) — always set by the operator:
   - `ifo.kubevirt.io/subject-name`
   - `ifo.kubevirt.io/subject-namespace`
   - `ifo.kubevirt.io/subject-kind`
   - `ifo.kubevirt.io/subject-uid`
   - `ifo.kubevirt.io/operation`
   - `ifo.kubevirt.io/component`
   - `ifo.kubevirt.io/ruleset`

Built-in labels cannot be overridden by static or dynamic labels.