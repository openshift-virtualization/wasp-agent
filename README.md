# Out-of-tree SWAP with WASP

WASP can be used in order to grant SWAP to certain containers using
an out-of-(kubernetes)-tree mechanism based on an OCI hook.

The design can be found in https://github.com/openshift/enhancements/pull/1630

## Prerequisites
- CRI-O as CRI
- runc as OCI
- Swap enabled

## Recommendations
- Set io latency for system.slice
- Disable swap in the system.slice

## Manage swap for kubernetes workloads
Wasp agent implmenetes the same policy as `swapBehavior: LimitedSwap` in kubernetes. It will allow limited swapping for burstable QoS workloads. The implementation of limit setting and the formula for limit calculation are the exact same as in k8s.

**Please note:** Same as in kubernetes, swap configuration will not be done for static pods, mirror pods, or critical system pods based on pod priority.


## Eviction

### Swap based eviction signals
- Utilization - How close are we to run out of swap?
- Traffic - How badly is swapping affecting the system?

### Pod selection for eviction
- Eviction doesn't target static pods, mirror pods, or critical system pods based on pod priority.

### Eviction order:
- Exceeding memory resource limits
- Exceeding resource requests
- Pod Priority
- The pod's resource usage relative to requests

Note: This is inspired by the [Kubernetes eviction order](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#pod-selection-for-kubelet-eviction)
, with an additional first criterion.


## Try it

### Deploy locally 

The development tools include a version of kubectl that you can use to communicate with the cluster.
A wrapper script to communicate with the cluster can be invoked using `./cluster-up/kubectl.sh`

```bash
$ mkdir $GOPATH/src/kubevirt.io && cd $GOPATH/src/kubevirt.io
$ git clone git@github.com:openshift-virtualization/wasp-agent.git && cd wasp-agent
$ make cluster-up
$ make cluster-sync
$ ./cluster-up/kubectl.sh .....
```

### Deploy on external kubernetes cluster

```bash
$ mkdir $GOPATH/src/wasp.io && cd $GOPATH/src/wasp.io
$ git clone git@github.com:openshift-virtualization/wasp-agent.git && cd wasp-agent
$ docker login <desired-registry> -u <username> -p <password>
$ export DOCKER_PREFIX=<desired-registry>/<desired-org> # i.e. quay.io/openshift-virtualization
$ export DOCKER_TAG=<desired-tag>
$ export KUBECONFIG=<kubeconfig-path>
$ export KUBEVIRT_PROVIDER=external 
$ make cluster-sync
```
### Deploy on Openshift

```bash
$ mkdir $GOPATH/src/wasp.io && cd $GOPATH/src/wasp.io
$ git clone git@github.com:openshift-virtualization/wasp-agent.git && cd wasp-agent
$ #
$ # Openshift Pre-requisits
$ #
$ # Note: KubeletConfig CRs are mutually exclusive
$ oc create -f manifests/openshift/kubelet-configuration-with-swap.yaml
$ oc wait mcp worker --for condition=Updated=True --timeout=300s
$ oc create -f manifests/openshift/machine-config-add-swap.yaml
$ oc wait mcp worker --for condition=Updated=True --timeout=300s
$ #
$ # WASP deployment
$ #
$ docker login <desired-registry> -u <username> -p <password>
$ export DOCKER_PREFIX=<desired-registry>/<desired-org> # i.e. quay.io/openshift-virtualization
$ export DOCKER_TAG=<desired-tag>
$ export KUBECONFIG=<kubeconfig-path>
$ export KUBEVIRT_PROVIDER=external 
$ make cluster-sync
```
