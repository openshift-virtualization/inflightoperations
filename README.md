# InFlightOperations

InFlightOperations is a Kubernetes operator that provides declarative, resource-agnostic operation tracking. It introduces two Custom Resource Definitions — `OperationRuleSet` and `InFlightOperation` — both in the `ifo.kubevirt.io/v1alpha1` API group.

An `OperationRuleSet` lets you declare CEL (Common Expression Language) expressions that target any Kubernetes resource type by Group/Version/Resource. When a watched resource matches a rule's CEL expression — for example, a VirtualMachine whose status indicates it is migrating — the operator automatically creates an `InFlightOperation` resource that tracks that operation through its lifecycle (Active → Completed). The operator uses dynamic informers so it only watches resource types that have active rulesets, and it caches compiled CEL programs for performance.

The core value is **operational visibility for cluster administrators**. Different components (KubeVirt, Forklift, CDI, OLM, etc.) can each publish their own `OperationRuleSet` definitions, and administrators can query `InFlightOperation` resources to get a unified view of what operations are currently in progress across the cluster — making it straightforward to understand cluster state, troubleshoot issues, and make informed decisions about when it is safe to perform maintenance or other disruptive actions.

## How It Works

**OperationRuleSet** — Declares what to watch and how to detect operations. Each ruleset targets a specific Kubernetes resource type (by Group/Version/Resource) and contains one or more CEL expressions that evaluate against those resources. When an expression matches, the operator knows an operation is in progress.

**InFlightOperation** — Created automatically by the operator when a rule matches. Tracks the operation through its lifecycle (Active → Completed) with timestamps and references back to the source resource and detecting ruleset.

### Example

An `OperationRuleSet` targeting KubeVirt VirtualMachines:

```yaml
apiVersion: ifo.kubevirt.io/v1alpha1
kind: OperationRuleSet
metadata:
  name: vm-lifecycle-rules
spec:
  component: kubevirt
  target:
    group: kubevirt.io
    version: v1
    resource: virtualmachines
  rules:
    - operation: Migrating
      expression: "has(object.status.printableStatus) && object.status.printableStatus == 'Migrating'"
    - operation: Starting
      expression: "has(object.status.printableStatus) && object.status.printableStatus == 'Starting'"
    - operation: Stopping
      expression: "has(object.status.printableStatus) && object.status.printableStatus == 'Stopping'"
```

When a VirtualMachine enters the "Migrating" state, the operator automatically creates an `InFlightOperation`:

```
$ kubectl get ifos
KIND               NAMESPACE   SUBJECT   OPERATION   PHASE    STARTED
VirtualMachine     default     my-vm     Migrating   Active   2025-01-15T10:30:00Z
```

Administrators can query InFlightOperation resources to see at a glance what operations are in progress across the cluster.

## Pre-Built Rules

The `rules/` directory includes ready-to-use rulesets for common resource types:

**KubeVirt**

| RuleSet | Target | Operations |
|---------|--------|------------|
| vm-lifecycle-rules | `kubevirt.io/v1/virtualmachines` | Migrating, Starting, Stopping, Provisioning, Terminating, WaitingForReceiver |
| vmi-rules | `kubevirt.io/v1/virtualmachineinstances` | Pending, Scheduling, Scheduled, Running, WaitingForSync, Provisioning, VCPUChange, MemoryChange |
| vmim-rules | `kubevirt.io/v1/virtualmachineinstancemigrations` | Pending, Scheduling, Scheduled, Running, PreparingTarget |
| kubevirt-rules | `kubevirt.io/v1/kubevirts` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing, Deleting, Upgrading, UpdateRollingOut |
| hco-rules | `hco.kubevirt.io/v1beta1/hyperconvergeds` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing |
| cdi-rules | `cdi.kubevirt.io/v1beta1/cdis` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing, Deleting, Upgrading |
| datavolume-rules | `cdi.kubevirt.io/v1beta1/datavolumes` | ImportScheduled, ImportInProgress, CloneScheduled, CloneInProgress, UploadScheduled, ExpansionInProgress, and 12 others |
| ssp-rules | `ssp.kubevirt.io/v1beta3/ssps` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing, Paused, Deleting, Upgrading |
| cnao-rules | `networkaddonsoperator.network.kubevirt.io/v1/networkaddonsconfigs` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing |
| hpp-rules | `hostpathprovisioner.kubevirt.io/v1beta1/hostpathprovisioners` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing |
| aaq-rules | `aaq.kubevirt.io/v1alpha1/aaqs` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing, Deleting, Upgrading |
| vm-snapshot-rules | `snapshot.kubevirt.io/v1beta1/virtualmachinesnapshots` | InProgress, Failed |
| vm-restore-rules | `snapshot.kubevirt.io/v1beta1/virtualmachinerestores` | InProgress, Failed |
| vm-clone-rules | `clone.kubevirt.io/v1beta1/virtualmachineclones` | SnapshotInProgress, CreatingTargetVM, RestoreInProgress, Failed |
| vm-export-rules | `export.kubevirt.io/v1beta1/virtualmachineexports` | Pending |

**OLM**

| RuleSet | Target | Operations |
|---------|--------|------------|
| csv-rules | `operators.coreos.com/v1alpha1/clusterserviceversions` | Pending, Installing, Replacing, Deleting, Failing |
| installplan-rules | `operators.coreos.com/v1alpha1/installplans` | Planning, Installing, RequiresApproval |
| subscription-rules | `operators.coreos.com/v1alpha1/subscriptions` | Unpacking, InstallPlanPending |

**OpenShift**

| RuleSet | Target | Operations |
|---------|--------|------------|
| clusterversion-rules | `config.openshift.io/v1/clusterversions` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing, NotUpgradeable, PartialUpdate |
| clusteroperator-rules | `config.openshift.io/v1/clusteroperators` | Deploying, Reconciling, ReconcilingDegraded, Healing, Failing, NotUpgradeable |
| machineconfigpool-rules | `machineconfiguration.openshift.io/v1/machineconfigpools` | Updating, UpdatingDegraded, Degraded, NodeDegraded, RenderDegraded, Paused, Building, BuildFailed |
| machine-rules | `machine.openshift.io/v1beta1/machines` | Provisioning, Provisioned, Deleting, Failed |
| machineset-rules | `machine.openshift.io/v1beta1/machinesets` | ScalingUp, ScalingDown |

**Generic**

| RuleSet | Target | Operations |
|---------|--------|------------|
| deployment-rules | `apps/v1/deployments` | Rollout |
| pvc-rules | `v1/persistentvolumeclaims` | Pending, Lost, Resizing, FilesystemResizePending |

## Configuration

The operator is configured via environment variables on the controller deployment:

| Variable | Default | Description |
|----------|---------|-------------|
| `DEBOUNCE_THRESHOLD` | `30s` | Grace period after an operation stops being detected before marking it Completed |
| `INFORMER_SYNC_TIMEOUT` | `30s` | Timeout for Kubernetes informer synchronization |
| `K8S_API_TIMEOUT` | `30s` | Timeout for Kubernetes API calls |
| `K8S_INFORMER_RESYNC` | `30s` | Resync period for informers |
| `RETAIN_COMPLETED_IFOS` | `false` | Whether to keep completed InFlightOperation resources |
| `REQUEUE_INTERVAL` | `60s` | Reconciliation requeue interval |
| `OPERATOR_VERSION` | — | Operator version identifier |

## CRD Reference

### OperationRuleSet

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.component` | string | No | Owning component name (e.g., "kubevirt", "olm") |
| `spec.target.group` | string | Yes | API group of the target resource |
| `spec.target.version` | string | Yes | API version of the target resource |
| `spec.target.resource` | string | Yes | Plural resource name |
| `spec.rules[].operation` | string | Yes | Name of the operation to detect |
| `spec.rules[].expression` | string | Yes | CEL expression evaluated against the resource |
| `spec.namespaces` | []string | No | Namespaces to watch (empty = all) |
| `spec.labels` | map | No | Static labels applied to created InFlightOperations |
| `spec.labelExpressions` | []object | No | CEL expressions for dynamic label computation; each entry has `key` (label key) and `expression` (CEL returning a string) |

Status fields: `conditions`, `watchActive`, `lastEvaluationTime`, `observedGeneration`.

### InFlightOperation

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.operation` | string | Yes | Name of the detected operation |
| `spec.ruleSet` | string | No | Name of the detecting OperationRuleSet |
| `spec.component` | string | No | Component name |
| `spec.subject.apiVersion` | string | Yes | API version of the subject resource |
| `spec.subject.kind` | string | Yes | Kind of the subject resource |
| `spec.subject.name` | string | Yes | Name of the subject resource |
| `spec.subject.namespace` | string | No | Namespace of the subject resource |
| `spec.subject.uid` | string | No | UID of the subject resource |
| `spec.subject.ownerReferences` | []OwnerReference | No | Owner references from the subject resource |

Status fields: `phase` (Active/Completed), `lastDetected`, `completed`, `detectedBy`, `subjectGeneration`, `conditions`.

Short names: `ifo`/`ifos` for InFlightOperation, `ors` for OperationRuleSet.

## kubectl Plugin

The `kubectl-ifo` plugin provides a CLI for viewing InFlightOperations. It is built as part of `make build` and can be installed to your PATH with `make install-plugin`.

### Commands

- `kubectl ifo list` — List InFlightOperations (default when no subcommand is given)
- `kubectl ifo tree` — Show operations as a grouped tree (component → namespace → resource hierarchy)
- `kubectl ifo get <name>` — Get a specific InFlightOperation
- `kubectl ifo summary` — Show summary statistics across all operations

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--namespace` | `-n` | Filter by subject namespace |
| `--component` | `-c` | Filter by component |
| `--kind` | `-k` | Filter by subject kind |
| `--operation` | `-o` | Filter by operation name |
| `--all-phases` | `-A` | Include completed IFOs (default: active only) |
| `--output` | `-O` | Output format: `table`, `wide`, `json`, `yaml` |
| `--selector` | `-l` | Label selector (passthrough) |
| `--color` | | Force ANSI color output |
| `--no-color` | | Disable ANSI color output |
| `--for` | | (tree only) Focus on a specific resource (`Kind/name`) |

### Example

```
$ kubectl ifo tree
kubevirt
  openshift-cnv
    HyperConverged/kubevirt-hyperconverged  Reconciling   3m
    └── KubeVirt/kubevirt-kubevirt           Upgrading     2m

olm
  openshift-operator-lifecycle-manager
    ClusterServiceVersion/my-operator.v2.0   Installing    45s
```

## Development

```bash
# Run tests
make test

# Run e2e tests (sets up a Kind cluster)
make setup-test-e2e
make test-e2e
make cleanup-test-e2e

# Build manager and plugin binaries
make build

# Install kubectl-ifo plugin to GOBIN
make install-plugin

# Build the container image
make docker-build

# Run linter
make lint

# Generate CRDs and deepcopy methods
make manifests generate
```

## Labels

InFlightOperation resources are labeled for easy querying:

**Subject labels**
- `ifo.kubevirt.io/subject-name` — Name of the subject resource
- `ifo.kubevirt.io/subject-namespace` — Namespace of the subject resource
- `ifo.kubevirt.io/subject-kind` — Kind of the subject resource
- `ifo.kubevirt.io/subject-uid` — UID of the subject resource

**Owner labels** (set when the subject has ownerReferences)
- `ifo.kubevirt.io/owner-uid` — UID of the first owner
- `ifo.kubevirt.io/owner-name` — Name of the first owner
- `ifo.kubevirt.io/owner-kind` — Kind of the first owner
- `ifo.kubevirt.io/owner-group` — API group of the first owner
- `ifo.kubevirt.io/owner-version` — API version of the first owner

**Operation labels**
- `ifo.kubevirt.io/operation` — Operation name
- `ifo.kubevirt.io/component` — Component name
- `ifo.kubevirt.io/ruleset` — RuleSet name

**Correlation labels** (for grouping related IFOs in tree views)
- `ifo.kubevirt.io/correlation-group` — Groups related IFOs together
- `ifo.kubevirt.io/correlation-role` — Role within a group (`root` or `child`)

Example query — find all active operations on a specific VM:

```bash
kubectl get ifos -l ifo.kubevirt.io/subject-name=my-vm,ifo.kubevirt.io/subject-namespace=default
```

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
