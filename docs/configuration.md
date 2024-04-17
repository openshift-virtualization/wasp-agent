# Configuring WASP agent for higher workload density

## Prerequisites

* `oc` is available
* Logged into cluster with `cluster-admin` role

## Procedure

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
   Create a `MachineConfig` according to the following
   [example](../manifests/machineconfig-add-swap.yaml).

5. Deploy alerting rules according to the following
   [example](../manifests/prometheus-rules.yaml).

6. Configure OpenShift Virtualization to use memory overcommit using
   the following example:

```console
$ oc patch --type=merge  -f [../manifests/prep-hco.yaml](../manifests/prep-hco.yaml) --patch-file [../manifests/prep-hco.yaml](../manifests/prep-hco.yaml)
```

## Verification

1. Validate the deployment
   TBD
2. Validate correctly configured Kubelet
   TBD
3. Validate correctly provisioned swap
   TBD
4. Validate OpenShift Virtualization configuration
   TBD

## Additional Resources

* https://github.com/openshift-virtualization/wasp-agent
