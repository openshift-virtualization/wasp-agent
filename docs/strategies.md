# Kubernetes and SWAP

# Overview

```
         +-------------+
         |             |
         + ~ ~ ~ ~ ~ ~ + - kube evict hard
         |             |
         + ~ ~ ~ ~ ~ ~ + - kube evict soft
         |             |
         |             |
         + ~ ~ ~ ~ ~ ~ + - kernel swap
         |             |
         |             |
         |             |
         |             |
         + - - - - - - +
         |   kube-res  |
         + - - - - - - +
         |   sys-res   |
         +-------------+
```

# Strategies
## Classic / Beta2
* Configuration
	* No system services configuration
	* Unlimited swap for burstable pods
* Pros
	* Simple
* Cons
	* What will get swapped?
	* Specific risk for system + kube, as they can be swapped, risking all workloads
* Notes
	* Containers and system services will participate in swap, depending on the environment configuration
	* Under memory pressure everything that has `memory.swap.max > 0` will be allowed to swap

## Ortho
* Configuration
	* By default, disable swap for _all_ system services (_everything_ owned by systemd incl kubelet)
	* By default, disable swap for _all_ kube pods
	* By default, leave swappiness at 60%
	* Swap is an extended resource, allocated to containers as requested, sets `memory.swap.max`
		* Other container cgroup `memory.` interfaces are not touced
	* Pros
		* Explicit noswap for all system services ("groundtruth")
		* Kubernetes with swap will behave by default like kubernetes without swap
			* All workloads will provide the same guarantees
		* Selected workloads will have the ability to swap
	* Cons
		* Per workload swap request
			* Opposite to hugepages
	* Notes
		* Containers with swap will still use memory as without swap
			* aka burst into free node memory
		* Only under memory pressure these workloads will be pushed to swap
			* Large `resource.limits.memory` will have an elevated eviction risk, but due to swap, by the time soft eviction is reached, it is expected that the `limit` range is
				* for idle worrkloads almost free
				* for busy workloads still utilized
			* therefore
				* idle workloads are expected to stay
				* busy workloads are expected to be evicted
        * T1
            * Defaults and beta2 leave too many processes with swap enabled, unable to turn it off
              all kind of process will eventually swap. no control.
