# OperationRuleSet Manual Verification Test Plan

## Overview

This test plan provides detailed instructions for manually verifying each OperationRuleSet definition. The goal is to confirm that CEL expressions correctly detect each operation by producing the expected state on a live cluster and verifying that the InFlightOperations controller creates (and completes) the corresponding InFlightOperation objects.

## Prerequisites

- An OpenShift cluster with OpenShift Virtualization (HCO) installed
- The InFlightOperations operator deployed and running
- All OperationRuleSet YAMLs from `rules/` applied to the cluster
- `kubectl` / `oc` configured with cluster-admin access
- A test namespace: `kubectl create ns ifo-test`
- CDI, KubeVirt, and the Machine API available

### Useful monitoring commands

```bash
# Watch all IFOs as they are created/updated
kubectl get ifo -w

# Watch IFOs for a specific subject
kubectl get ifo -l ifo.kubevirt.io/subject-name=<name> -w

# Watch IFOs for a specific operation
kubectl get ifo --field-selector spec.operation=<operation> -w

# Check which rulesets are active
kubectl get ors -o wide
```

---

## 1. Deployment Rules (`deployment-rules`)

**Target:** apps/v1/deployments

### 1.1 Rollout

**Steps:**
1. Create a deployment:
   ```bash
   kubectl create deployment ifo-test-deploy --image=nginx:1.25 --replicas=3 -n ifo-test
   ```
2. Wait for it to stabilize (all 3 replicas available), confirm no IFO exists.
3. Trigger a rollout by updating the image:
   ```bash
   kubectl set image deployment/ifo-test-deploy nginx=nginx:1.26 -n ifo-test
   ```
4. Immediately check for an IFO:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-deploy
   ```

**Expected:** An IFO with `operation: Rollout` is created while `updatedReplicas`, `readyReplicas`, or `availableReplicas` are less than `spec.replicas`. The IFO should complete once the rollout finishes.

**Cleanup:** `kubectl delete deployment ifo-test-deploy -n ifo-test`

---

## 2. PVC Rules (`pvc-rules`)

**Target:** core/v1/persistentvolumeclaims

### 2.1 Pending

**Steps:**
1. Create a PVC requesting a storage class that does not exist:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: ifo-test-pending-pvc
     namespace: ifo-test
   spec:
     accessModes: [ReadWriteOnce]
     storageClassName: nonexistent-storage-class
     resources:
       requests:
         storage: 1Gi
   EOF
   ```
2. Verify the PVC is stuck in `Pending`:
   ```bash
   kubectl get pvc ifo-test-pending-pvc -n ifo-test
   ```
3. Check for a corresponding IFO:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-pending-pvc
   ```

**Expected:** An IFO with `operation: Pending` exists for the PVC.

**Cleanup:** `kubectl delete pvc ifo-test-pending-pvc -n ifo-test`

### 2.2 Resizing / FilesystemResizePending

**Prerequisites:** A StorageClass that supports volume expansion (`allowVolumeExpansion: true`). Verify:
```bash
kubectl get sc -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.allowVolumeExpansion}{"\n"}{end}'
```

**Steps:**
1. Create a PVC with the expandable storage class and a pod that mounts it:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: ifo-test-resize-pvc
     namespace: ifo-test
   spec:
     accessModes: [ReadWriteOnce]
     storageClassName: <expandable-sc>
     resources:
       requests:
         storage: 1Gi
   ---
   apiVersion: v1
   kind: Pod
   metadata:
     name: ifo-test-resize-pod
     namespace: ifo-test
   spec:
     containers:
     - name: app
       image: busybox
       command: [sleep, infinity]
       volumeMounts:
       - name: data
         mountPath: /data
     volumes:
     - name: data
       persistentVolumeClaim:
         claimName: ifo-test-resize-pvc
   EOF
   ```
2. Wait for the pod to be Running and the PVC to be Bound.
3. Expand the PVC:
   ```bash
   kubectl patch pvc ifo-test-resize-pvc -n ifo-test -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
   ```
4. Immediately watch the PVC conditions:
   ```bash
   kubectl get pvc ifo-test-resize-pvc -n ifo-test -o jsonpath='{.status.conditions}' | jq .
   ```
5. Check for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-resize-pvc
   ```

**Expected:** Depending on the storage driver:
- An IFO with `operation: Resizing` appears while the `Resizing` condition is `True`.
- An IFO with `operation: FilesystemResizePending` may appear if the filesystem resize requires a pod restart.
- Both IFOs should complete once the resize finishes.

**Cleanup:** `kubectl delete pod ifo-test-resize-pod -n ifo-test && kubectl delete pvc ifo-test-resize-pvc -n ifo-test`

### 2.3 Lost

**Note:** This requires deleting a PV while a PVC is bound to it. This is a destructive operation on storage.

**Steps:**
1. Create a PVC and wait for it to bind.
2. Identify the PV: `kubectl get pvc ifo-test-lost-pvc -n ifo-test -o jsonpath='{.spec.volumeName}'`
3. Set the PV's reclaim policy to Retain: `kubectl patch pv <pv-name> -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'`
4. Delete the PV: `kubectl delete pv <pv-name>`
5. The PVC should transition to `Lost`.
6. Check for IFO with `operation: Lost`.

**Expected:** An IFO with `operation: Lost` appears.

**Cleanup:** Delete the PVC.

---

## 3. DataVolume Rules (`datavolume-rules`)

**Target:** cdi.kubevirt.io/v1beta1/datavolumes

### 3.1 ImportInProgress

**Steps:**
1. Create a DataVolume that imports from a URL (use a large image to give time to observe):
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cdi.kubevirt.io/v1beta1
   kind: DataVolume
   metadata:
     name: ifo-test-import
     namespace: ifo-test
   spec:
     source:
       http:
         url: "https://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img"
     storage:
       accessModes: [ReadWriteOnce]
       resources:
         requests:
           storage: 1Gi
   EOF
   ```
2. Monitor the DV phase: `kubectl get dv ifo-test-import -n ifo-test -w`
3. Check for IFOs at each phase transition:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-import
   ```

**Expected:** IFOs should appear for the phases the DV passes through. Typically: `Pending` -> `ImportScheduled` -> `ImportInProgress` -> (complete). Each IFO should be created when the DV enters that phase and completed when it leaves.

**Cleanup:** `kubectl delete dv ifo-test-import -n ifo-test`

### 3.2 CloneInProgress

**Steps:**
1. Ensure a source PVC with data exists (e.g., from the import test above, or create a new one).
2. Create a DataVolume that clones from the source:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cdi.kubevirt.io/v1beta1
   kind: DataVolume
   metadata:
     name: ifo-test-clone
     namespace: ifo-test
   spec:
     source:
       pvc:
         namespace: ifo-test
         name: ifo-test-import
     storage:
       accessModes: [ReadWriteOnce]
       resources:
         requests:
           storage: 1Gi
   EOF
   ```
3. Watch for IFOs through the clone phases.

**Expected:** IFOs for `CloneScheduled` and/or `CloneInProgress` (or `CSICloneInProgress` / `SmartClonePVCInProgress` depending on the storage driver).

**Cleanup:** `kubectl delete dv ifo-test-clone -n ifo-test`

### 3.3 WaitForFirstConsumer

**Steps:**
1. Identify a StorageClass with `volumeBindingMode: WaitForFirstConsumer`:
   ```bash
   kubectl get sc -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.volumeBindingMode}{"\n"}{end}'
   ```
2. Create a DataVolume using that StorageClass with no consumer pod.
3. Observe the DV entering the `WaitForFirstConsumer` phase.

**Expected:** An IFO with `operation: WaitForFirstConsumer` appears.

### 3.4 ImportScheduled

**Steps:**
1. Create a DataVolume that imports from an HTTP source (same as 3.1).
2. The DV transitions through `ImportScheduled` before `ImportInProgress`. Watch:
   ```bash
   kubectl get dv ifo-test-import -n ifo-test -w
   ```

**Expected:** An IFO with `operation: ImportScheduled` appears briefly before the importer pod starts.

### 3.5 Pending / PVCBound

**Steps:**
1. Create a DataVolume. Before the DV controller begins processing, the DV may briefly sit in `Pending`.
2. After the PVC is created and binds but before the operation (import/clone/upload) begins, the DV may enter `PVCBound`.

**Expected:** IFOs with `operation: Pending` and/or `operation: PVCBound` appear as transient states. These are typically very brief.

### 3.6 Clone variants (CloneScheduled, SnapshotForSmartCloneInProgress, CloneFromSnapshotSourceInProgress, SmartClonePVCInProgress, CSICloneInProgress)

**Prerequisites:** A source PVC with data. The specific clone variant depends on the storage backend:
- **CSICloneInProgress** — storage driver supports CSI volume cloning
- **SmartClonePVCInProgress** — CDI smart-clone via host-assisted copy
- **SnapshotForSmartCloneInProgress** / **CloneFromSnapshotSourceInProgress** — CDI snapshot-based smart clone (requires VolumeSnapshot support)

**Steps:**
1. Create a clone DataVolume (same as 3.2).
2. Watch the DV phase:
   ```bash
   kubectl get dv ifo-test-clone -n ifo-test -o jsonpath='{.status.phase}' -w
   ```
3. The specific clone phase that appears depends on the storage driver's capabilities and CDI's clone strategy selection.

**Expected:** An IFO with `operation: CloneScheduled` appears first, followed by one of the clone-in-progress variants (`CSICloneInProgress`, `SmartClonePVCInProgress`, `SnapshotForSmartCloneInProgress`, or `CloneFromSnapshotSourceInProgress`) depending on the storage backend.

### 3.7 Upload (UploadScheduled, UploadReady)

**Prerequisites:** `virtctl` CLI available.

**Steps:**
1. Create a DataVolume with an upload source:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cdi.kubevirt.io/v1beta1
   kind: DataVolume
   metadata:
     name: ifo-test-upload
     namespace: ifo-test
   spec:
     source:
       upload: {}
     storage:
       accessModes: [ReadWriteOnce]
       resources:
         requests:
           storage: 1Gi
   EOF
   ```
2. Watch the DV phase:
   ```bash
   kubectl get dv ifo-test-upload -n ifo-test -w
   ```
3. The DV enters `UploadScheduled` while the upload server pod starts, then `UploadReady` when it's ready to receive data.
4. Upload an image:
   ```bash
   virtctl image-upload dv ifo-test-upload --image-path=/path/to/disk.img --no-create -n ifo-test
   ```

**Expected:** IFOs with `operation: UploadScheduled` and `operation: UploadReady` appear in sequence.

**Cleanup:** `kubectl delete dv ifo-test-upload -n ifo-test`

### 3.8 PendingPopulation / PrepClaimInProgress

**Steps:**
1. `PendingPopulation` occurs when a DataVolume uses a population source (e.g., a DataSource with a snapshot source) and the population has not yet started.
2. `PrepClaimInProgress` occurs during early PVC preparation before the main data operation begins.
3. These are typically transient phases. To observe, create a DataVolume that references a DataSource:
   ```bash
   # Requires an existing DataSource (e.g., from a golden image namespace)
   kubectl get datasource -A
   ```

**Expected:** IFOs with `operation: PendingPopulation` or `operation: PrepClaimInProgress` appear as transient states.

### 3.9 ExpansionInProgress / NamespaceTransferInProgress / RebindInProgress

These are advanced CDI operations:
- **ExpansionInProgress** — DV's PVC is being expanded. Trigger by patching a DV's storage request to a larger size.
- **NamespaceTransferInProgress** — CDI is transferring an object across namespaces. Trigger by using CDI's ObjectTransfer API:
  ```bash
  cat <<EOF | kubectl apply -f -
  apiVersion: cdi.kubevirt.io/v1beta1
  kind: ObjectTransfer
  metadata:
    name: ifo-test-transfer
  spec:
    source:
      kind: DataVolume
      namespace: ifo-test
      name: ifo-test-import
    target:
      namespace: ifo-test-target
  EOF
  ```
- **RebindInProgress** — PVC is being rebound to a different PV during certain CDI operations.

**Expected:** IFOs appear for whichever phase the DV enters. These operations require specific CDI features and storage capabilities.

---

## 4. VirtualMachine Rules (`vm-lifecycle-rules`)

**Target:** kubevirt.io/v1/virtualmachines

### 4.1 Starting / Provisioning

**Steps:**
1. Create a VM (stopped):
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: kubevirt.io/v1
   kind: VirtualMachine
   metadata:
     name: ifo-test-vm
     namespace: ifo-test
   spec:
     running: false
     template:
       spec:
         domain:
           resources:
             requests:
               memory: 128Mi
           devices:
             disks:
             - name: containerdisk
               disk:
                 bus: virtio
         volumes:
         - name: containerdisk
           containerDisk:
             image: quay.io/kubevirt/cirros-container-disk-demo:latest
   EOF
   ```
2. Start the VM:
   ```bash
   virtctl start ifo-test-vm -n ifo-test
   ```
3. Watch for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-vm -w
   ```

**Expected:** IFOs with `operation: Provisioning` and/or `operation: Starting` appear as the VM transitions through `printableStatus` values.

### 4.2 Stopping

**Steps:**
1. With the VM running from 4.1:
   ```bash
   virtctl stop ifo-test-vm -n ifo-test
   ```
2. Watch for IFOs.

**Expected:** An IFO with `operation: Stopping` appears.

### 4.3 Migrating

**Prerequisites:** At least 2 schedulable worker nodes.

**Steps:**
1. Start the VM if not already running.
2. Initiate a live migration:
   ```bash
   virtctl migrate ifo-test-vm -n ifo-test
   ```
3. Watch for IFOs.

**Expected:** An IFO with `operation: Migrating` appears while the migration is in progress and completes when the migration finishes.

### 4.4 Terminating

**Steps:**
1. With a running VM:
   ```bash
   kubectl delete vm ifo-test-vm -n ifo-test
   ```
2. Watch for IFOs during the teardown:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-vm -w
   ```

**Expected:** An IFO with `operation: Terminating` appears while the VM's `printableStatus` is `Terminating` (during VMI shutdown and resource cleanup).

### 4.5 WaitingForReceiver

This occurs during migration when the target node hasn't created the receiver pod yet. It is transient and may be difficult to catch. A heavily-loaded cluster or constrained node resources may make this more observable.

**Cleanup:** `kubectl delete vm ifo-test-vm -n ifo-test`

---

## 5. VirtualMachineInstance Rules (`vmi-rules`)

**Target:** kubevirt.io/v1/virtualmachineinstances

### 5.1 Phase transitions (Pending -> Scheduling -> Scheduled -> Running)

**Steps:**
1. Start a VM (from test 4.1). The VMI is created automatically.
2. Watch VMI phases:
   ```bash
   kubectl get vmi -n ifo-test -w
   ```
3. Check for IFOs at each phase:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-namespace=ifo-test
   ```

**Expected:** IFOs for `Pending`, `Scheduling`, `Scheduled`, and `Running` appear as the VMI transitions. Each completes when the VMI leaves that phase.

### 5.2 VCPUChange / MemoryChange (hot-plug)

**Prerequisites:** A VM with hot-plug CPU/memory enabled (requires `LiveMigrate` eviction strategy and appropriate feature gates).

**Steps:**
1. Create and start a VM with CPU hot-plug enabled.
2. Hot-plug additional vCPUs:
   ```bash
   kubectl patch vm ifo-test-vm -n ifo-test --type merge -p '{"spec":{"template":{"spec":{"domain":{"cpu":{"cores":2}}}}}}'
   ```
3. Watch for IFOs with `operation: VCPUChange`.

**Expected:** An IFO with `operation: VCPUChange` appears when the `HotVCPUChange` condition is True on the VMI.

**Note:** Memory hot-plug (`MemoryChange`) follows the same pattern but patches the memory request.

### 5.3 WaitingForSync

**Steps:**
1. The `WaitingForSync` phase is a transient VMI state that may occur during certain internal synchronization windows (e.g., after a migration or when virt-handler is resyncing with a VMI).
2. This phase is extremely brief and difficult to catch under normal circumstances. To verify:
   ```bash
   # Watch VMI phases with timestamps to catch brief transitions
   kubectl get vmi -n ifo-test -w --output-watch-events
   ```
3. Alternatively, verify the CEL expression is structurally correct by confirming it follows the same `status.phase` pattern as the other VMI lifecycle rules.

**Expected:** An IFO with `operation: WaitingForSync` would appear if the VMI enters this phase. In practice, this is rarely observed during manual testing.

---

## 6. VirtualMachineInstanceMigration Rules (`vmim-rules`)

**Target:** kubevirt.io/v1/virtualmachineinstancemigrations

### 6.1 Migration lifecycle (Pending -> Scheduling -> Scheduled -> PreparingTarget -> Running)

**Steps:**
1. Start a VM and initiate a migration (from test 4.3).
2. Watch the VMIM object:
   ```bash
   kubectl get vmim -n ifo-test -w
   ```
3. Check for IFOs with the VMIM as subject at each phase.

**Expected:** IFOs appear for each phase the VMIM passes through. The `PreparingTarget` phase may be brief. The `Running` IFO should complete when the migration finishes.

---

## 7. VirtualMachineSnapshot Rules (`vm-snapshot-rules`)

**Target:** snapshot.kubevirt.io/v1beta1/virtualmachinesnapshots

### 7.1 InProgress

**Steps:**
1. With a running or stopped VM:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: snapshot.kubevirt.io/v1beta1
   kind: VirtualMachineSnapshot
   metadata:
     name: ifo-test-snap
     namespace: ifo-test
   spec:
     source:
       apiGroup: kubevirt.io
       kind: VirtualMachine
       name: ifo-test-vm
   EOF
   ```
2. Watch the snapshot phase:
   ```bash
   kubectl get vmsnapshot ifo-test-snap -n ifo-test -w
   ```
3. Check for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-snap
   ```

**Expected:** An IFO with `operation: InProgress` appears while `status.phase` is `InProgress`. It completes when the snapshot succeeds.

### 7.2 Failed

**Steps:**
1. Create a snapshot targeting a VM that doesn't exist:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: snapshot.kubevirt.io/v1beta1
   kind: VirtualMachineSnapshot
   metadata:
     name: ifo-test-snap-fail
     namespace: ifo-test
   spec:
     source:
       apiGroup: kubevirt.io
       kind: VirtualMachine
       name: nonexistent-vm
   EOF
   ```
2. Wait for the snapshot to fail, then check for an IFO with `operation: Failed`.

**Expected:** An IFO with `operation: Failed` appears.

**Cleanup:** `kubectl delete vmsnapshot --all -n ifo-test`

---

## 8. VirtualMachineRestore Rules (`vm-restore-rules`)

**Target:** snapshot.kubevirt.io/v1beta1/virtualmachinerestores

### 8.1 InProgress

**Prerequisites:** A successful VirtualMachineSnapshot from test 7.1.

**Steps:**
1. Stop the VM if running: `virtctl stop ifo-test-vm -n ifo-test`
2. Create a restore:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: snapshot.kubevirt.io/v1beta1
   kind: VirtualMachineRestore
   metadata:
     name: ifo-test-restore
     namespace: ifo-test
   spec:
     target:
       apiGroup: kubevirt.io
       kind: VirtualMachine
       name: ifo-test-vm
     virtualMachineSnapshotName: ifo-test-snap
   EOF
   ```
3. Watch for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=ifo-test-restore
   ```

**Expected:** An IFO with `operation: InProgress` appears while the restore is running, and completes when it succeeds.

**Cleanup:** `kubectl delete vmrestore ifo-test-restore -n ifo-test`

---

## 9. VirtualMachineClone Rules (`vm-clone-rules`)

**Target:** clone.kubevirt.io/v1beta1/virtualmachineclones

### 9.1 Clone lifecycle (SnapshotInProgress -> CreatingTargetVM -> RestoreInProgress)

**Steps:**
1. With a stopped VM available:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: clone.kubevirt.io/v1beta1
   kind: VirtualMachineClone
   metadata:
     name: ifo-test-clone
     namespace: ifo-test
   spec:
     source:
       apiGroup: kubevirt.io
       kind: VirtualMachine
       name: ifo-test-vm
     target:
       apiGroup: kubevirt.io
       kind: VirtualMachine
       name: ifo-test-vm-clone
   EOF
   ```
2. Watch the clone phases:
   ```bash
   kubectl get vmclone ifo-test-clone -n ifo-test -w
   ```
3. Check for IFOs at each phase.

**Expected:** IFOs appear for `SnapshotInProgress`, `CreatingTargetVM`, and `RestoreInProgress` as the clone progresses through its phases. Each completes when the clone moves to the next phase.

### 9.2 Failed

**Steps:**
1. Create a clone targeting a nonexistent VM source, or a clone that would exceed storage quotas.
2. Verify an IFO with `operation: Failed` appears.

**Cleanup:** `kubectl delete vmclone ifo-test-clone -n ifo-test && kubectl delete vm ifo-test-vm-clone -n ifo-test`

---

## 10. VirtualMachineExport Rules (`vm-export-rules`)

**Target:** export.kubevirt.io/v1beta1/virtualmachineexports

### 10.1 Pending

**Steps:**
1. Create an export for an existing VM:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: export.kubevirt.io/v1beta1
   kind: VirtualMachineExport
   metadata:
     name: ifo-test-export
     namespace: ifo-test
   spec:
     source:
       apiGroup: kubevirt.io
       kind: VirtualMachine
       name: ifo-test-vm
   EOF
   ```
2. Watch the export phase:
   ```bash
   kubectl get vmexport ifo-test-export -n ifo-test -o jsonpath='{.status.phase}'
   ```
3. Check for an IFO while the export is in `Pending` phase.

**Expected:** An IFO with `operation: Pending` appears while the export is initializing. It completes when the export transitions to `Ready`.

**Cleanup:** `kubectl delete vmexport ifo-test-export -n ifo-test`

---

## 11. HCO Operator Rules (`hco-rules`)

**Target:** hco.kubevirt.io/v1beta1/hyperconvergeds

These rules use the standard Available/Progressing/Degraded condition pattern. The HCO object is a singleton (`kubevirt-hyperconverged` in the HCO namespace).

### 11.1 Reconciling

**Steps:**
1. After a fresh HCO install or when the cluster is stable, check:
   ```bash
   kubectl get hco kubevirt-hyperconverged -n openshift-cnv -o jsonpath='{.status.conditions}' | jq .
   ```
2. If Available=True, Progressing=True, Degraded=False, an IFO with `operation: Reconciling` should exist.

**Expected:** Reconciling IFO present during normal operation when Progressing is True.

### 11.2 Deploying / ReconcilingDegraded / Healing / Failing

These states require the HCO to be in specific condition combinations that typically occur during initial install, component failures, or recovery. The full condition matrix for the shared Available/Progressing/Degraded pattern (used by HCO, KubeVirt, CDI, CNAO, SSP, HPP, AAQ, ClusterVersion, ClusterOperator) is:

| Operation | Available | Progressing | Degraded |
|---|---|---|---|
| Deploying | False | True | False |
| Reconciling | True | True | False |
| ReconcilingDegraded | True | True | True |
| Healing | False | True | True |
| Failing | False | False | True |

They can be triggered by:
- **Deploying:** Observe during initial HCO installation.
- **ReconcilingDegraded:** The operator is available and making progress but also reporting degradation (e.g., a non-critical component is unhealthy). This is the combination of Available=True, Progressing=True, Degraded=True.
- **Healing:** Delete a component deployment (e.g., `kubectl delete deployment virt-api -n openshift-cnv`) and observe the recovery.
- **Failing:** Introduce a persistent failure condition (e.g., invalid configuration in the HCO CR).

**Note:** Be cautious with these tests on shared clusters, as they affect the entire virtualization stack.

---

## 12. KubeVirt Operator Rules (`kubevirt-rules`)

**Target:** kubevirt.io/v1/kubevirts

### 12.1 Standard conditions (Deploying/Reconciling/etc.)

Same pattern as HCO rules (section 11). Check the KubeVirt CR:
```bash
kubectl get kubevirt kubevirt -n openshift-cnv -o jsonpath='{.status.conditions}' | jq .
```

### 12.2 Upgrading

**Steps:**
1. Check the version fields:
   ```bash
   kubectl get kubevirt kubevirt -n openshift-cnv -o jsonpath='{.status.observedKubeVirtVersion}  {.status.targetKubeVirtVersion}'
   ```
2. This IFO appears during an OpenShift Virtualization upgrade when `observedKubeVirtVersion != targetKubeVirtVersion`.

**Expected:** During an upgrade, an IFO with `operation: Upgrading` appears.

### 12.3 UpdateRollingOut

**Steps:**
1. During or after an upgrade, check:
   ```bash
   kubectl get kubevirt kubevirt -n openshift-cnv -o jsonpath='{.status.outdatedVirtualMachineInstanceWorkloads}'
   ```
2. If Available=True and `outdatedVirtualMachineInstanceWorkloads > 0`, an IFO with `operation: UpdateRollingOut` should exist.

**Expected:** UpdateRollingOut IFO appears while VMI workloads are still running on the old version.

### 12.4 Deleting

**Steps:**
1. This can only be tested by deleting the KubeVirt CR (which removes the entire virtualization stack). Not recommended on shared clusters. Verify the CEL expression is structurally correct by inspecting the status phase during a controlled teardown.

---

## 13. CDI Operator Rules (`cdi-rules`)

**Target:** cdi.kubevirt.io/v1beta1/cdis

Same condition pattern as HCO/KubeVirt (section 11). Additionally:

### 13.1 Deleting / Upgrading

- **Upgrading:** Observed during an OpenShift Virtualization upgrade when the CDI CR's `status.phase` transitions to `Upgrading`.
- **Deleting:** Observed when the CDI CR is being deleted.

Check the CDI CR:
```bash
kubectl get cdi cdi -n openshift-cnv -o jsonpath='{.status.phase}  {.status.conditions}' | jq .
```

---

## 14. CNAO Rules (`cnao-rules`)

**Target:** networkaddonsoperator.network.kubevirt.io/v1/networkaddonsconfigs

Same condition pattern as HCO/KubeVirt. Check:
```bash
kubectl get networkaddonsconfig cluster -o jsonpath='{.status.conditions}' | jq .
```

---

## 15. SSP Rules (`ssp-rules`)

**Target:** ssp.kubevirt.io/v1beta3/ssps

### 15.1 Standard conditions

Same pattern as HCO/KubeVirt. Check:
```bash
kubectl get ssp ssp-kubevirt-hyperconverged -n openshift-cnv -o jsonpath='{.status.conditions}' | jq .
```

### 15.2 Paused

**Steps:**
1. Pause the SSP:
   ```bash
   kubectl patch ssp ssp-kubevirt-hyperconverged -n openshift-cnv --type merge -p '{"spec":{"paused":true}}'
   ```
   **Note:** On an HCO-managed cluster, HCO may override this. Test on a standalone SSP deployment or verify the field is set.
2. Verify `status.paused` is `true`:
   ```bash
   kubectl get ssp ssp-kubevirt-hyperconverged -n openshift-cnv -o jsonpath='{.status.paused}'
   ```
3. Check for IFO with `operation: Paused`.

**Expected:** IFO with `operation: Paused` appears.

**Cleanup:** Unpause with `{"spec":{"paused":false}}`.

---

## 16. HPP Rules (`hpp-rules`)

**Target:** hostpathprovisioner.kubevirt.io/v1beta1/hostpathprovisioners

Same condition pattern. Check:
```bash
kubectl get hostpathprovisioner -o jsonpath='{.items[0].status.conditions}' | jq .
```

**Note:** HPP may not be installed on all clusters. Skip if not present.

---

## 17. AAQ Rules (`aaq-rules`)

**Target:** aaq.kubevirt.io/v1alpha1/aaqs

Same condition pattern. Check:
```bash
kubectl get aaq -o jsonpath='{.items[0].status.conditions}' | jq .
```

**Note:** AAQ may not be installed on all clusters. Skip if not present.

---

## 18. OLM Rules

### 18.1 CSV Rules (`csv-rules`)

**Target:** operators.coreos.com/v1alpha1/clusterserviceversions

**Steps:**
1. Install a simple operator from OperatorHub to observe CSV phase transitions:
   ```bash
   # Use the OpenShift console or create a Subscription for a lightweight operator
   ```
2. Watch CSV phases during installation:
   ```bash
   kubectl get csv -n <namespace> -w
   ```
3. Verify IFOs appear for `Pending`, `Installing` phases.

**Expected:** IFOs for the CSV phases the operator passes through during installation.

### 18.1.1 Replacing

**Steps:**
1. Install an operator at a specific version, then update the Subscription to a newer channel or approve an update.
2. Watch for the old CSV entering `Replacing` phase:
   ```bash
   kubectl get csv -n <namespace> -w
   ```
3. The old CSV transitions to `Replacing` while the new CSV is being installed.

**Expected:** An IFO with `operation: Replacing` appears for the old CSV.

### 18.1.2 Deleting

**Steps:**
1. Uninstall an operator by deleting its Subscription and CSV:
   ```bash
   kubectl delete subscription <name> -n <namespace>
   kubectl delete csv <name> -n <namespace>
   ```
2. Watch for the CSV entering `Deleting` phase.

**Expected:** An IFO with `operation: Deleting` appears briefly while the CSV is being cleaned up.

### 18.1.3 Failing

**Steps:**
1. Install an operator that will fail (e.g., one that requires a CRD or RBAC that doesn't exist, or create a CSV manually with an invalid deployment spec):
   ```bash
   # The CSV will enter the Failed phase if its deployment cannot be created
   kubectl get csv -n <namespace> -o jsonpath='{.status.phase}'
   ```
2. Verify an IFO with `operation: Failing` appears.

**Expected:** An IFO with `operation: Failing` appears when `status.phase == "Failed"`.

### 18.2 InstallPlan Rules (`installplan-rules`)

**Target:** operators.coreos.com/v1alpha1/installplans

**Steps:**
1. Create a Subscription with `installPlanApproval: Manual`:
   ```bash
   # This causes InstallPlans to be created in RequiresApproval state
   ```
2. Verify an IFO with `operation: RequiresApproval` appears.
3. Approve the InstallPlan and verify IFOs for `Planning` and `Installing` appear.

### 18.3 Subscription Rules (`subscription-rules`)

**Target:** operators.coreos.com/v1alpha1/subscriptions

**Steps:**
1. During an operator install, watch subscription conditions:
   ```bash
   kubectl get subscription <name> -n <namespace> -o jsonpath='{.status.conditions}' | jq .
   ```
2. Look for `BundleUnpacking` and `InstallPlanPending` conditions.
3. Verify corresponding IFOs appear.

---

## 19. MachineConfigPool Rules (`machineconfigpool-rules`)

**Target:** machineconfiguration.openshift.io/v1/machineconfigpools

### 19.1 Updating

**Steps:**
1. Apply a MachineConfig that triggers a pool update (e.g., add a kernel argument):
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: machineconfiguration.openshift.io/v1
   kind: MachineConfig
   metadata:
     labels:
       machineconfiguration.openshift.io/role: worker
     name: ifo-test-mc
   spec:
     config:
       ignition:
         version: 3.2.0
       storage:
         files:
         - contents:
             source: data:,ifo-test
           mode: 0644
           path: /etc/ifo-test
   EOF
   ```
2. Watch the MCP:
   ```bash
   kubectl get mcp worker -w
   ```
3. Check for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=worker
   ```

**Expected:** An IFO with `operation: Updating` appears while the `Updating` condition is True. Nodes will be cordoned, drained, and rebooted one at a time. This may take 10+ minutes per node.

**Cleanup:** `kubectl delete mc ifo-test-mc` (this triggers another MCP update to roll back).

### 19.2 Paused

**Steps:**
1. Pause the worker pool:
   ```bash
   kubectl patch mcp worker --type merge -p '{"spec":{"paused":true}}'
   ```
2. Check for IFO with `operation: Paused`.

**Expected:** IFO with `operation: Paused` appears.

**Cleanup:** `kubectl patch mcp worker --type merge -p '{"spec":{"paused":false}}'`

### 19.3 UpdatingDegraded / Degraded / NodeDegraded / RenderDegraded / Building / BuildFailed

These conditions are triggered by specific failure modes:
- **UpdatingDegraded:** An update is in progress (Updating=True) but the pool is also degraded (Degraded=True). This differs from plain `Degraded` which means the pool is NOT updating (Updating=False) but is degraded. For example, a node fails to apply a new MachineConfig during a rolling update.
- **Degraded:** The pool is not updating but a node has failed to apply its MachineConfig (Updating=False, Degraded=True).
- **NodeDegraded:** A specific node reports degradation.
- **RenderDegraded:** The render controller cannot generate a rendered MachineConfig.
- **Building / BuildFailed:** On-cluster builds (MCO build controller) for custom OS images.

These are difficult to trigger intentionally without breaking the cluster. Verify by:
1. Checking the MCP conditions on a running cluster:
   ```bash
   kubectl get mcp worker -o jsonpath='{.status.conditions}' | jq .
   ```
2. If any of these conditions are True, verify a corresponding IFO exists.

---

## 20. ClusterVersion Rules (`clusterversion-rules`)

**Target:** config.openshift.io/v1/clusterversions

### 20.1 Standard conditions

Check the ClusterVersion object:
```bash
kubectl get clusterversion version -o jsonpath='{.status.conditions}' | jq .
```

On a stable cluster, Available=True, Progressing=False typically. Progressing=True would be seen during an OpenShift upgrade.

### 20.2 NotUpgradeable

**Steps:**
1. Check: `kubectl get clusterversion version -o jsonpath='{.status.conditions}' | jq '.[] | select(.type=="Upgradeable")'`
2. If `Upgradeable=False`, an IFO with `operation: NotUpgradeable` should exist.

**Note:** Operators can set Upgradeable=False when they aren't ready for an upgrade. This is common during certain maintenance windows.

### 20.3 PartialUpdate

**Steps:**
1. During an OpenShift upgrade, check:
   ```bash
   kubectl get clusterversion version -o jsonpath='{.status.history[0].state}'
   ```
2. If the value is `Partial`, an IFO with `operation: PartialUpdate` should exist.

**Expected:** PartialUpdate IFO during an in-progress upgrade.

---

## 21. ClusterOperator Rules (`clusteroperator-rules`)

**Target:** config.openshift.io/v1/clusteroperators

### 21.1 Standard conditions + NotUpgradeable

**Steps:**
1. List all ClusterOperators and their conditions:
   ```bash
   kubectl get co
   ```
2. For any operator with `AVAILABLE=False` and `PROGRESSING=True`, verify a `Deploying` or `Healing` IFO exists (depending on `DEGRADED` status).
3. For any operator with `UPGRADEABLE=False` (often seen on `kube-apiserver`, `etcd`, etc.), verify a `NotUpgradeable` IFO exists.

**Expected:** IFOs matching the current condition states of each ClusterOperator.

---

## 22. Machine Rules (`machine-rules`)

**Target:** machine.openshift.io/v1beta1/machines

### 22.1 Provisioning / Provisioned

**Steps:**
1. Scale up a MachineSet to create a new Machine:
   ```bash
   # Identify a MachineSet
   kubectl get machineset -n openshift-machine-api
   # Scale it up by 1
   kubectl scale machineset <name> -n openshift-machine-api --replicas=<current+1>
   ```
2. Watch the new Machine:
   ```bash
   kubectl get machine -n openshift-machine-api -w
   ```
3. Check for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-namespace=openshift-machine-api
   ```

**Expected:** An IFO with `operation: Provisioning` appears while the Machine is in the `Provisioning` phase. Then an IFO with `operation: Provisioned` when the instance exists but the node isn't ready yet. Both complete as the Machine transitions to `Running`.

### 22.2 Deleting

**Steps:**
1. Scale the MachineSet back down:
   ```bash
   kubectl scale machineset <name> -n openshift-machine-api --replicas=<original>
   ```
2. Watch for a Machine entering the `Deleting` phase.
3. Check for an IFO with `operation: Deleting`.

**Expected:** IFO with `operation: Deleting` appears during decommission.

### 22.3 Failed

A Failed Machine occurs when the cloud provider cannot provision the instance (e.g., quota exceeded, invalid machine spec). This can be triggered by creating a Machine with an invalid instance type, but is risky on shared clusters.

---

## 23. MachineSet Rules (`machineset-rules`)

**Target:** machine.openshift.io/v1beta1/machinesets

### 23.1 ScalingUp

**Steps:**
1. Scale up a MachineSet (from test 22.1).
2. Immediately check for IFOs:
   ```bash
   kubectl get ifo -l ifo.kubevirt.io/subject-name=<machineset-name>
   ```
3. Verify `spec.replicas > status.readyReplicas` on the MachineSet.

**Expected:** An IFO with `operation: ScalingUp` appears while the MachineSet has more desired replicas than ready replicas. It completes when all new Machines become Ready.

### 23.2 ScalingDown

**Steps:**
1. Scale down the MachineSet (from test 22.2).
2. There will be a brief period where `spec.replicas < status.readyReplicas` (the old Machine is still Ready while being drained/deleted).
3. Check for an IFO with `operation: ScalingDown`.

**Expected:** An IFO with `operation: ScalingDown` appears during the scale-down window.

---

## Negative Test Cases

These verify that IFOs are NOT created when operations are not in progress.

### N1. Stable Deployment produces no Rollout IFO
1. Create a deployment and wait for it to stabilize.
2. Verify no IFO with `operation: Rollout` exists for that deployment.

### N2. Bound PVC produces no Pending IFO
1. Create a PVC with a valid StorageClass.
2. Wait for it to bind.
3. Verify no IFO with `operation: Pending` exists.

### N3. Successful snapshot produces no InProgress IFO
1. After a VirtualMachineSnapshot completes successfully, verify no IFO with `operation: InProgress` remains (it should have been completed).

### N4. Stable MachineSet produces no Scaling IFO
1. Check an existing MachineSet where `spec.replicas == status.readyReplicas`.
2. Verify no ScalingUp or ScalingDown IFO exists.

### N5. Running Machine produces no Provisioning IFO
1. Check a Machine in `Running` phase.
2. Verify no Provisioning, Provisioned, or Deleting IFO exists.

---

## IFO Lifecycle Verification

For any test case above, also verify the IFO lifecycle:

1. **Creation:** IFO is created with `status.phase: Active` when the operation begins.
2. **DetectedBy:** `status.detectedBy` lists the OperationRuleSet name that detected the operation.
3. **Completion:** When the operation ends, the IFO transitions to `status.phase: Completed` with a `status.completed` timestamp.
4. **Cleanup:** After the debounce threshold (default 30s), completed IFOs are deleted by the controller.
5. **Subject reference:** `spec.subject` correctly identifies the resource (apiVersion, kind, name, namespace, uid).
6. **Labels:** IFOs have standard labels (`ifo.kubevirt.io/subject-name`, `ifo.kubevirt.io/subject-namespace`, `ifo.kubevirt.io/operation`, `ifo.kubevirt.io/ruleset`).

---

## Test Environment Cleanup

```bash
kubectl delete ns ifo-test
# Remove any MachineConfigs created during testing
kubectl delete mc ifo-test-mc --ignore-not-found
# Verify no test IFOs remain
kubectl get ifo | grep ifo-test
```
