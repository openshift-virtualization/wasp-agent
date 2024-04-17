# Configuring higher workload density

You can configure a higher VM workload densityin your cluster
by over-committing memory resources (RAM).

While over-committed memory can lead to a higher workload density, at
the same time this will lead to some side-effects:

- Lower workload performance on a highly utilized system

Some workloads are more suited for higher workload density than
others, for example:

- Many similar workloads
- Underutilized workloads

## Configuring higher workload density with the wasp-agent

[wasp-agent] is an component to enable an OpenShift cluster to assign
SWAP resources to burstable VM workloads only.

### Prerequisites

* `oc` is available
* Logged into cluster with `cluster-admin` role
* A defined memory over-commit ratio. By default: 150%

### Procedure

> [!NOTE]
> The `wasp-agent` will deploy an OCI hook in order to enable
> swap usage for containres on the node level.
> The low-level nature requires the `DaemonSet` to be privileged.

1. Create a privileged service account:

```console
$ oc adm new-project wasp
$ oc create sa -n waspi wasp
$ oc adm policy add-cluster-role-to-user cluster-admin -z wasp
$ oc adm policy add-scc-to-user -n wasp privileged -z wasp
```

2. Deploy `wasp-agent`
   Create a `DaemonSet` according to the following
   [example](../manifests/ds.yaml).

3. Configure `Kubelet` to permit swap
   Create a `KubeletConfiguration` according to the following
   [example](../manifests/kubelet-configuration-with-swap.yaml).

4. Create `MachineConfig` to provision swap

> [!IMPORTANT]
> In order to have enough swap for the worst case scenario, it must
> be ensured to have at least as much swap space provisioned as RAM
> is being over-committed.
> The amount of swap space to be provisioned on a node must
> be calculated according to the following formula:
>
>     NODE_SWAP_SPACE = NODE_RAM * MEMORY_OVER_COMMIT_RATIO
>
> Example:
>
>     NODE_SWAP_SPACE = 16 GB * 150%
>                     = 16 GB * 0.5
>                     =  8 GB

   Create a `MachineConfig` according to the following
   [example](../manifests/machineconfig-add-swap.yaml).

5. Deploy alerting rules according to the following
   [example](../manifests/prometheus-rules.yaml).

6. Configure OpenShift Virtualization to use memory overcommit using
   the following [example](../manifests/prep-hco.yaml):

```console
$ oc patch --type=merge \
  -f <../manifests/prep-hco.yaml> \
  --patch-file <../manifests/prep-hco.yaml>
```

> [!NOTE]
> After applying all configurations all `MachineConfigPool`
> roll-outs have to complete before the feature is fully available.
>
>     oc wait mcp worker --for condition=Updated=True
>

### Verification

1. Validate the deployment
   TBD
2. Validate correctly configured Kubelet
   TBD
3. Validate correctly provisioned swap:

       $ oc get nodes -l node-role.kubernetes.io/worker
       # Select a node from the provided list

       $ oc debug node/<selected-node> -- free -m

4. Validate OpenShift Virtualization configuration
   TBD

### Additional Resources

[wasp-agent]: https://github.com/openshift-virtualization/wasp-agent
FPR: Free-Page Reporting
KSM:
