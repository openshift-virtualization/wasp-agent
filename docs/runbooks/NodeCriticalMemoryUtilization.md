# NodeCriticalMemoryUtilization

## Meaning

This alert is triggered when memory utilization (RAM + Swap) on a node exceeds 105% of the total usable RAM. This indicates critically high memory pressure on the node.

## Impact

The performance of the node is impacted. This can lead to containers restarts, severe performance degradation and potential crashes.

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
