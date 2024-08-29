# NodeHighSwapActivity

## Meaning

This alert is triggered when the rate of swap out and swap in exceeds 200 in both operations in the last minute. 

## Impact

The performance of the node is impacted. This can lead to containers restarts, performance degradation and system instability.

## Diagnosis

To diagnose the cause of this alert, the following steps can be taken:

1. **Check Running Processes**: Use commands like `top` in the node terminal, to identify memory-intensive processes.
2. **Analyze Workloads**: Review memory-intensive workloads on the node. Focus on burstable pods and VMs workloads.

## Mitigation

To mitigate the impact of this alert, consider the following actions:

1. Setup memory resource limits in order to limit individual workloads.
2. Reduce the number of workloads running on an affected node using eviction.
3. Add more nodes to the cluster to distribute memory load.
4. Optimize memory usage of applications.
