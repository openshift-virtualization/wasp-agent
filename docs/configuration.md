# Configuring higher workload density

You can configure a higher VM workload and pod workload density in your cluster 
by over-committing memory resources (RAM).

While over-committed memory can lead to a higher workload density, at
the same time this will lead to some side-effects:

- Lower workload performance on a highly utilized system

Some workloads are more suited for higher workload density than
others, for example:

- Many similar workloads
- Underutilized workloads

## Configuring higher workload density with the wasp-agent

[wasp-agent]  is a component that enables an OpenShift cluster to assign 
SWAP resources to burstable pod and VM workloads. Additionally, wasp-agent 
is responsible for managing pod evictions when the system is heavily loaded and nodes are at risk.

SWAP usage is supported on worker nodes only.

### Prerequisites

* `oc` is available
* Logged into cluster with `cluster-admin` role
* A defined memory over-commit ratio. By default: 150%
* A worker pool

### Procedure

> [!NOTE]
> The `wasp-agent` will deploy an OCI hook and periodically 
> verifies the swap limit to ensure it is set correctly in order to enable
> swap usage for containers on the node level.
> The low-level nature requires the `DaemonSet` to be privileged.

1. Configure `Kubelet` to permit swap
   Create a `KubeletConfiguration` according to the following
   [example](../manifests/openshift/kubelet-configuration-with-swap.yaml).

2. Wait for the worker nodes to sync with the new config:
```console
$ oc wait mcp worker --for condition=Updated=True --timeout=-1s
```

3. Create `MachineConfig` to provision swap according to the following [example](../manifests/openshift/machineconfig-add-swap.yaml)

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

Create a `MachineConfig` according to the following
[example](../manifests/openshift/machineconfig-add-swap.yaml).

3. Create a privileged service account:

```console
$ oc adm new-project wasp
$ oc create sa -n wasp wasp
$ oc create clusterrolebinding wasp --clusterrole=cluster-admin --serviceaccount=wasp:wasp
$ oc adm policy add-scc-to-user -n wasp privileged -z wasp
```

4. Wait for the worker nodes to sync with the new config:
```console
$ oc wait mcp worker --for condition=Updated=True --timeout=-1s
```

5. Deploy `wasp-agent`
   Create a `DaemonSet` according to the following
   [example](../manifests/openshift/ds.yaml).

> [!IMPORTANT]
> The wasp-agent  manages pod eviction when the system is heavily loaded and
> nodes are at risk. Eviction will be triggered if any of the following conditions occur:
> 1. High Swap I/O Traffic:
>* `averageSwapInPerSecond > maxAverageSwapInPagesPerSecond`
   and<br>
   `averageSwapOutPerSecond > maxAverageSwapOutPagesPerSecond`<br><br>
   This condition is triggered when swap-related I/O traffic
   is excessively high.
   The default values for `maxAverageSwapInPagesPerSecond` and
   `maxAverageSwapOutPagesPerSecond` are both set to 1000, averaged
   over a 30-second interval by default.
> 2. High Swap Utilization:
> * `nodeWorkingSet + nodeSwapUsage < totalNodeMemory + totalSwapMemory * thresholdFactor `<br><br>
    This condition is triggered when swap utilization is excessively high.
    This depends on the `NODE_SWAP_SPACE` setting in the machine configuration from
    step 3 or when the current virtual memory usage exceeds the factored threshold<br><br>
>
>
> `maxAverageSwapInPagesPerSecond` and `maxAverageSwapOutPagesPerSecond`
> can be adjusted by modifying the values of the
> `MAX_AVERAGE_SWAP_IN_PAGES_PER_SECOND` and `MAX_AVERAGE_SWAP_OUT_PAGES_PER_SECOND`.<br><br>
>
>
> The high swap utilization threshold factor can be adjusted using ``SWAP_UTILIZATION_THRESHOLD_FACTOR`` variable. For example, having a node with 16G RAM and 8G swap, with a threshold factor of 0.8 the threshold would be 16 + 8*0.8 = 16 + 6.4 = 22.4G <br><br>
> Additionally, the `AVERAGE_WINDOW_SIZE_SECONDS` environment
> variable determines the time frame for calculating the average.


5. Deploy alerting rules according to the following
   [example](../manifests/openshift/prometheus-rule.yaml) and
   add the cluster-monitoring label to the wasp namespace.
```console
$ oc label namespace wasp openshift.io/cluster-monitoring="true"
```

6. Configure OpenShift Virtualization to use memory overcommit using

   a. Via the OpenShift Console: <br>
       **Virtualization -> Overview -> Settings -> General Settings -> Memory Density** <br>
       ![image](https://github.com/user-attachments/assets/07c02c7c-0cd6-4377-9119-a8f3b7a58695)

   b. Alternatively via the CLI: [HCO example](../manifests/openshift/hco-set-memory-overcommit.yaml):

```console
$ oc patch --type=merge \
  -f <../manifests/openshift/hco-set-memory-overcommit.yaml> \
  --patch-file <../manifests/openshift/hco-set-memory-overcommit.yaml>
```

### Upgrade path
For users of wasp-agent v1.0, which lacks LimitedSwap and eviction support, here is the upgrade path:
1. Adjust KubeletConfig:
   If you have installed the KubeletConfiguration object from version v1.0, you need to update it. 
   This can be done by applying the updated [KubeletConfiguration example](../manifests/openshift/kubelet-configuration-with-swap.yaml).
```console
$ oc replace -f <../manifests/openshift/kubelet-configuration-with-swap.yaml>
```
2. Update DaemonSet:
   Apply the updated DaemonSet by using the provided [DaemonSet example](../manifests/openshift/ds.yaml).
```console
$ oc apply -f <../manifests/openshift/ds.yaml>
```
3. Update monitoring object:
```console
$ oc delete AlertingRule wasp-alerts -nopenshift-monitoring
$ oc create -f <../manifests/openshift/prometheus-rule.yaml>
```

### Verification

1. Validate the deployment
   TBD
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
   TBD

### Additional Resources

[wasp-agent]: https://github.com/openshift-virtualization/wasp-agent
FPR: Free-Page Reporting
KSM:
