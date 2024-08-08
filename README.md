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

## Wasp Agent: configuring Kubernetes containers
- Set a specific amount of swap for each burstable pod to match Kubernetes limited swap settings.

**Please note:** swap configuration will not be done for static pods, mirror pods, or critical system pods based on pod priority.

### Pod selection for eviction
- Eviction targets non-static pods, mirror pods, or critical system pods based on pod priority.
  Pods in namespaces beginning with "openshift" or "kube-system" are excluded from eviction.

## Eviction

### Swap based eviction signals
- Utilization - How close are we to run out of swap?
- Traffic - How badly is swapping affecting the system?
- Overcommit ratio - How over committed is the system?

### Pod selection for eviction
- Eviction doesn't target static pods, mirror pods, or critical system pods based on pod priority.
  Pods in namespaces beginning with "openshift" or "kube-system" are excluded from eviction.

### Eviction order:
- Exceeding memory resource limits
- Exceeding resource requests
- Pod Priority
- The pod's resource usage relative to requests

Note: This is inspired by the [Kubernetes eviction order](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#pod-selection-for-kubelet-eviction)
, with an additional first criterion.


## Build

### Deploy it on your cluster

```bash
$ mkdir $GOPATH/src/wasp.io && cd $GOPATH/src/wasp.io
$ git clone git@github.com:openshift-virtualization/wasp-agent.git && cd wasp-agent
$ export KUBECONFIG=<kubeconfig-path>
$ export KUBEVIRT_PROVIDER=external 
$ make cluster-sync
```
If you are interested in pushing the images to a remote repository:
```bash
$ mkdir $GOPATH/src/wasp.io && cd $GOPATH/src/wasp.io
$ git clone git@github.com:openshift-virtualization/wasp-agent.git && cd wasp-agent
$ export DOCKER_PREFIX=<desired-registry>
$ export DOCKER_TAG=<desired-tag>
$ export KUBECONFIG=<kubeconfig-path>
$ export KUBEVIRT_PROVIDER=external 
$ make cluster-sync
```

### Deploy it with our CI system

Wasp includes a self-contained development and test environment.  
We use Docker to build, and we provide a simple way to get a test 
cluster up and running on your laptop. 
The development tools include a version of kubectl that you can use to communicate with the cluster. 
A wrapper script to communicate with the cluster can be invoked using ./cluster-up/kubectl.sh.

```bash
$ mkdir $GOPATH/src/kubevirt.io && cd $GOPATH/src/kubevirt.io
$ git clone git@github.com:openshift-virtualization/wasp-agent.git && cd wasp-agent
$ make cluster-up
$ make cluster-sync
$ ./cluster-up/kubectl.sh .....
```
