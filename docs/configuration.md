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

1. #### Create a privileged service account:

```console
$ oc adm new-project wasp
$ oc create sa -n wasp wasp
$ oc adm policy add-cluster-role-to-user cluster-admin -z wasp
$ oc adm policy add-scc-to-user -n wasp privileged -z wasp
```

2. #### Deploy `wasp-agent` <br>
* ##### Determine wasp-agent image pull URL:
```console
$ OCP_VERSION=$(oc get clusterversion | awk 'NR==2' |cut -d' ' -f4 | cut -d'-' -f1)
$ oc get csv kubevirt-hyperconverged-operator.v${OCP_VERSION} -nopenshift-cnv -ojson | jq '.spec.relatedImages[] | select(.name|test(".*wasp-agent.*")) | .image'
```
* ##### Create a `DaemonSet` with the relevant image URL according to the following [example](../manifests/ds.yaml).


3. #### Create a `KubeletConfiguration` according to the following [example](../manifests/kubelet-configuration-with-swap.yaml).

4. #### Create `MachineConfig` to provision swap according to the following [example](../manifests/machineconfig-add-swap.yaml)

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

5. #### Create a `MachineConfig` according to the following [example](../manifests/machineconfig-add-swap.yaml).

6. #### Deploy alerting rules according to the following [example](../manifests/prometheus-rules.yaml).

7. #### Configure OpenShift Virtualization to use memory overcommit using

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
