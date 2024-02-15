# NodeSwapping

## Meaning

The alert `NodeSwapping` is triggered when the node is starting to
swap memory pages in and out for a certain amount of time.

## Impact

The performance of the node is impacted. This can lead to health
or liveness check failures and container restarts.

## Diagnosis

Check the OpenShift monitoring dashboard in order to identify the
nodes which are swapping by looking at the
`node_memory_SwapFree_bytes` metric.

## Mitigation

The resolution depends on the particular issue reported in the logs.

Common mitigations include

- Setup memory reousrce limits in order to limit individual workloads
- Reduce the number of workloads running on a node
