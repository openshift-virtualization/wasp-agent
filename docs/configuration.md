# Configuring higher workload density

You can configure a higher VM workload density in your cluster
by over-committing memory resources (RAM).

While over-committed memory can lead to a higher workload density, at
the same time this will lead to some side-effects:

- Lower workload performance on a highly utilized system

Some workloads are more suited for higher workload density than
others, for example:

- Many similar workloads
- Underutilized workloads

## Configuring higher workload density with the wasp-agent

[wasp-agent] is a component to enable an OpenShift cluster to assign
SWAP resources to burstable VM workloads only.

SWAP usage is supported on worker nodes only.

### Prerequisites

* `oc` is available
* Logged into cluster with `cluster-admin` role
* A defined memory over-commit ratio. By default: 150%
* A worker pool

### Procedure

> [!NOTE]
> The `wasp-agent` will deploy an OCI hook in order to enable
> swap usage for containers on the node level.
> The low-level nature requires the `DaemonSet` to be privileged.

> [!NOTE]
> For Red Hat release please replace the container image URL in the example with the following:
> ```console
> registry.redhat.io/container-native-virtualization/wasp-agent-rhel9:latest
> ```

1. Create a privileged service account:

```console
$ oc adm new-project wasp
$ oc project wasp
$ oc create sa -n wasp wasp
$ oc adm policy add-cluster-role-to-user cluster-admin -n wasp -z wasp
$ oc adm policy add-scc-to-user -n wasp privileged -z wasp
```

2. Deploy `wasp-agent`.
   Create a `DaemonSet` according to the following
   [example](../manifests/ds.yaml).

3. Configure `Kubelet` to permit swap
   Create a `KubeletConfiguration` according to the following
   [example](../manifests/kubelet-configuration-with-swap.yaml).

4. Wait for the worker nodes to finish syncing before proceeding to the next step.
   you can use the following command:
   ```console
   $ oc wait mcp worker --for condition=Updated=True
   ```

5. Create `MachineConfig` to provision swap according to the following [example](../manifests/machineconfig-add-swap.yaml)

> [!CAUTION]
> All worker nodes in a cluster are expected to have the same
> RAM to SWAP ratio. If there ratio differs between nodes in a cluster
> then workloads are at risk of getting killed during live migration.
> A mitigation can be to limit workloads to nodes with the same RAM to
> SWAP ratio by using ie. Taints and Tolerations, or LabelSelectors.

> [!IMPORTANT]
> In order to have enough swap for the worst case scenario, it must
> be ensured to have at least as much swap space provisioned as RAM
> is being over-committed.
> The amount of swap space to be provisioned on a node must
> be calculated according to the following formula:
>
>     NODE_SWAP_SPACE = NODE_RAM * (MEMORY_OVER_COMMIT_PERCENT / 100% - 1)
>
> Example:
>
>     NODE_SWAP_SPACE = 16 GB * (150% / 100% - 1)
>                     = 16 GB * (1.5 - 1)
>                     = 16 GB * (0.5)
>                     =  8 GB

6. Deploy alerting rules according to the following
   [example](../manifests/prometheus-rules.yaml).

7. Configure OpenShift Virtualization to use memory overcommit using

   a. the OpenShift Console
   b. the following [HCO example](../manifests/hco-set-memory-overcommit.yaml):

```console
$ oc patch --type=merge \
  -f <../manifests/hco-set-memory-overcommit.yaml> \
  --patch-file <../manifests/hco-set-memory-overcommit.yaml>
```

> [!NOTE]
> After applying all configurations all `MachineConfigPool`
> roll-outs have to complete before the feature is fully available.
>
>     $ oc wait mcp worker --for condition=Updated=True
>

### Verification

1. Validate the deployment

       $ oc rollout status ds wasp-agent -n wasp --timeout 2m
       daemon set "wasp-agent" successfully rolled out

2. Validate correctly provisioned swap by running:

       $ oc get nodes -l node-role.kubernetes.io/worker
       # Select a node from the provided list

       $ oc debug node/<selected-node> -- free -m

   Should show an amoutn larger than zero for swap, similar to:

                      total        used        free      shared  buff/cache   available
       Mem:           31846       23155        1044        6014       14483        8690
       Swap:           8191        2337        5854


3. Validate OpenShift Virtualization memory overcommitment configuration
   by running:

       $ oc get -n openshift-cnv HyperConverged kubevirt-hyperconverged -o jsonpath="{.spec.higherWorkloadDensity.memoryOvercommitPercentage}"
       150

    The returned value (in this case `150`) should match the value you
    have configured earlier on.

4. Validate Virtual Machine memory overcommitment
   Create a VM with as much memory as the node has. Without memory
   overcommit the VM will not be scheduled. Only with memory
   overcommit the VM will be scheduled and can run.

### Additional Resources

[wasp-agent]: https://github.com/openshift-virtualization/wasp-agent
FPR: Free-Page Reporting
KSM:
