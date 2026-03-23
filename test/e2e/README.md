# End-to-End Tests

These tests validate the InFlightOperations operator against a live Kubernetes cluster. They exercise the full operator loop: deploying the operator, creating OperationRuleSets, watching resources, evaluating CEL rules, creating and completing InFlightOperations, and cleaning up.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed
- Docker running (for building the operator image)
- `kubectl` configured

## Running

```bash
make test-e2e
```

This will:
1. Create a Kind cluster (`inflightoperations-test-e2e`)
2. Build the operator image and load it into Kind
3. Install CRDs and deploy the operator
4. Run the test suite
5. Tear down the Kind cluster

## Test Scenarios

### Infrastructure (kubebuilder boilerplate)

| Test | What it verifies |
|------|-----------------|
| Manager should run successfully | Controller pod starts and reaches Running phase |
| Metrics endpoint serving | Metrics endpoint responds with HTTP 200 via RBAC-authenticated curl |

### OperationRuleSet Reconciliation

| Test | What it verifies |
|------|-----------------|
| ORS reconciles to Ready | Applying `rules/deployment_operationrule.yaml` results in `Ready=True`, `watchActive=true`, and a finalizer |
| Invalid CEL expression rejected | An ORS with bad CEL gets `InvalidRule` condition, not `Ready` |
| ORS deletion cleanup | Deleting an ORS runs the finalizer and fully removes it |

### InFlightOperation Lifecycle

| Test | What it verifies |
|------|-----------------|
| IFO created on rollout | Creating a Deployment triggers a `Rollout` IFO with correct labels and subject fields |
| IFO completed on rollout finish | After the Deployment becomes available, the IFO is completed or deleted |
| IFOs cleaned up on subject delete | Deleting the Deployment deletes all associated IFOs |

### Labels

| Test | What it verifies |
|------|-----------------|
| Static labels | An ORS with `spec.labels` produces IFOs carrying those labels |
| Label expressions | An ORS with `spec.labelExpressions` produces IFOs with dynamically-evaluated labels |

### Namespace Filtering

| Test | What it verifies |
|------|-----------------|
| Namespace scoping | An ORS with `spec.namespaces` only creates IFOs for Deployments in the specified namespace |

## Design Notes

- **Target resource**: All tests use Deployments (apps/v1) because they are available in any Kubernetes cluster without additional CRDs.
- **Test image**: `registry.k8s.io/pause:3.10` is used as the container image — it starts instantly and uses minimal resources.
- **Timing**: Tests use Ginkgo's `Eventually` with a 2-minute default timeout and 1-second polling interval. Some assertions use `Consistently` to verify something does *not* happen.
- **Cleanup**: Each test cleans up its own resources. The `AfterAll` block removes any remaining test resources.
- **IFO lifecycle**: Since `RetainCompletedIFOs` defaults to `false`, completed IFOs are deleted. Completion tests check for either deletion or `Completed` phase to handle both configurations.
