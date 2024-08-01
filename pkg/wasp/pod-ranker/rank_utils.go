package pod_ranker

import (
	stats_collector "github.com/openshift-virtualization/wasp-agent/pkg/wasp/stats-collector"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	v1resource "k8s.io/kubernetes/pkg/api/v1/resource"
	"sort"
)

// GetResourceLimitsQuantity finds and returns the limit quantity for a specific resource.
func GetResourceLimitsQuantity(pod *v1.Pod, resourceName v1.ResourceName) resource.Quantity {
	requestQuantity := resource.Quantity{}

	switch resourceName {
	case v1.ResourceCPU:
		requestQuantity = resource.Quantity{Format: resource.DecimalSI}
	case v1.ResourceMemory, v1.ResourceStorage, v1.ResourceEphemeralStorage:
		requestQuantity = resource.Quantity{Format: resource.BinarySI}
	default:
		requestQuantity = resource.Quantity{Format: resource.DecimalSI}
	}

	for _, container := range pod.Spec.Containers {
		if rQuantity, ok := container.Resources.Limits[resourceName]; ok {
			requestQuantity.Add(rQuantity)
		}
	}

	for _, container := range pod.Spec.InitContainers {
		if rQuantity, ok := container.Resources.Limits[resourceName]; ok {
			if requestQuantity.Cmp(rQuantity) < 0 {
				requestQuantity = rQuantity.DeepCopy()
			}
		}
	}

	// Add overhead for running a pod
	// to the total Limits if the resource total is non-zero
	if pod.Spec.Overhead != nil {
		if podOverhead, ok := pod.Spec.Overhead[resourceName]; ok && !requestQuantity.IsZero() {
			requestQuantity.Add(podOverhead)
		}
	}

	return requestQuantity
}

// memory compares pods by largest consumer of memory relative to request.
func memory(summary summaryFunc) cmpFunc {
	return func(p1, p2 *v1.Pod) int {
		p1Summary, p1Found := summary(p1)
		p2Summary, p2Found := summary(p2)
		if !p1Found || !p2Found {
			// prioritize evicting the pod for which no stats were found
			return cmpBool(!p1Found, !p2Found)
		}

		// adjust p1, p2 usage relative to the request (if any)
		p1Memory := memoryAndSwapUsage(*p1Summary.MemoryWorkingSetBytes, *p1Summary.MemorySwapCurrentBytes)
		p1Request := v1resource.GetResourceRequestQuantity(p1, v1.ResourceMemory)
		p1Memory.Sub(p1Request)

		p2Memory := memoryAndSwapUsage(*p2Summary.MemoryWorkingSetBytes, *p2Summary.MemorySwapCurrentBytes)
		p2Request := v1resource.GetResourceRequestQuantity(p2, v1.ResourceMemory)
		p2Memory.Sub(p2Request)

		// prioritize evicting the pod which has the larger consumption of memory
		return p2Memory.Cmp(*p1Memory)
	}
}

// priority compares pods by Priority, if priority is enabled.
func priority(p1, p2 *v1.Pod) int {
	priority1 := corev1helpers.PodPriority(p1)
	priority2 := corev1helpers.PodPriority(p2)
	if priority1 == priority2 {
		return 0
	}
	if priority1 > priority2 {
		return 1
	}
	return -1
}

// statsFunc returns the usage stats if known for an input pod.
type summaryFunc func(pod *v1.Pod) (stats_collector.PodSummary, bool)

// exceedMemory compares whether or not pods' memory usage exceeds their requests
func exceedMemoryRequests(summary summaryFunc) cmpFunc {
	return func(p1, p2 *v1.Pod) int {
		p1Stats, p1Found := summary(p1)
		p2Stats, p2Found := summary(p2)
		if !p1Found || !p2Found {
			// prioritize evicting the pod for which no stats were found
			return cmpBool(!p1Found, !p2Found)
		}

		p1MemoryAndSwapSum := memoryAndSwapUsage(*p1Stats.MemoryWorkingSetBytes, *p1Stats.MemorySwapCurrentBytes)
		p2MemoryAndSwapSum := memoryAndSwapUsage(*p2Stats.MemoryWorkingSetBytes, *p2Stats.MemorySwapCurrentBytes)

		/*
			log.Log.Infof(fmt.Sprintf("Debug: ______________________________________________________________________________________________________________"))
			log.Log.Infof(fmt.Sprintf("Debug: p1Name: %v p1Namespace: %v p2Name: %v p2Namespace: %v", p1.Name, p1.Namespace, p2.Name, p2.Namespace))
			log.Log.Infof(fmt.Sprintf("Debug: p1MemoryAndSwapSum:%v p2MemoryAndSwapSum: %v", p1MemoryAndSwapSum, p2MemoryAndSwapSum))
			log.Log.Infof(fmt.Sprintf("Debug: GetResourceRequestQuantity(p1, v1.ResourceMemory):%v GetResourceRequestQuantity(p2, v1.ResourceMemory): %v", v1resource.GetResourceRequestQuantity(p1, v1.ResourceMemory), v1resource.GetResourceRequestQuantity(p2, v1.ResourceMemory)))
		*/
		p1ExceedsRequests := p1MemoryAndSwapSum.Cmp(v1resource.GetResourceRequestQuantity(p1, v1.ResourceMemory)) == 1
		p2ExceedsRequests := p2MemoryAndSwapSum.Cmp(v1resource.GetResourceRequestQuantity(p2, v1.ResourceMemory)) == 1

		// prioritize evicting the pod which exceeds its requests
		return cmpBool(p1ExceedsRequests, p2ExceedsRequests)
	}
}

// exceedMemory compares whether or not pods' memory usage exceeds their requests
func exceedMemoryLimits(summary summaryFunc) cmpFunc {
	return func(p1, p2 *v1.Pod) int {
		p1Stats, p1Found := summary(p1)
		p2Stats, p2Found := summary(p2)
		if !p1Found || !p2Found {
			// prioritize evicting the pod for which no stats were found
			return cmpBool(!p1Found, !p2Found)
		}
		p1HasContainerExceedMemoryLimits := hasContainerExceedMemoryLimits(p1, p1Stats)
		p2HasContainerExceedMemoryLimits := hasContainerExceedMemoryLimits(p2, p2Stats)

		// prioritize evicting the pod which exceeds its requests
		return cmpBool(p1HasContainerExceedMemoryLimits, p2HasContainerExceedMemoryLimits)
	}
}

func hasContainerExceedMemoryLimits(pod *v1.Pod, summary stats_collector.PodSummary) bool {
	for _, container := range pod.Spec.Containers {
		if rQuantity, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
			memoryAndSwapSumQuantity := memoryAndSwapUsage(*summary.Containers[container.Name].MemoryWorkingSetBytes, *summary.Containers[container.Name].MemorySwapCurrentBytes)
			if memoryAndSwapSumQuantity.Cmp(rQuantity) == 1 {
				return true
			}
		}
	}

	for _, container := range pod.Spec.InitContainers {
		if rQuantity, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
			memoryAndSwapSumQuantity := memoryAndSwapUsage(*summary.Containers[container.Name].MemoryWorkingSetBytes, *summary.Containers[container.Name].MemorySwapCurrentBytes)
			if memoryAndSwapSumQuantity.Cmp(rQuantity) == 1 {
				return true
			}
		}
	}
	return false
}

// cmpBool compares booleans, placing true before false
func cmpBool(a, b bool) int {
	if a == b {
		return 0
	}
	if !b {
		return -1
	}
	return 1
}

// memoryAndSwapUsage converts working set into a resource quantity.
func memoryAndSwapUsage(memoryWorkingSetBytes uint64, memorySwapCurrentBytes uint64) *resource.Quantity {
	workingsetUsage := int64(memoryWorkingSetBytes)
	swapUsage := int64(memorySwapCurrentBytes)

	totalQuantity := resource.NewQuantity(workingsetUsage, resource.BinarySI)
	totalQuantity.Add(*resource.NewQuantity(swapUsage, resource.BinarySI))
	return totalQuantity
}

// Cmp compares p1 and p2 and returns:
//
//	-1 if p1 <  p2
//	 0 if p1 == p2
//	+1 if p1 >  p2
type cmpFunc func(p1, p2 *v1.Pod) int

// multiSorter implements the Sort interface, sorting changes within.
type multiSorter struct {
	pods []*v1.Pod
	cmp  []cmpFunc
}

// OrderedBy returns a Sorter that sorts using the cmp functions, in order.
// Call its Sort method to sort the data.
func orderedBy(cmp ...cmpFunc) *multiSorter {
	return &multiSorter{
		cmp: cmp,
	}
}

// Len is part of sort.Interface.
func (ms *multiSorter) Len() int {
	return len(ms.pods)
}

// Swap is part of sort.Interface.
func (ms *multiSorter) Swap(i, j int) {
	ms.pods[i], ms.pods[j] = ms.pods[j], ms.pods[i]
}

// Less is part of sort.Interface.
func (ms *multiSorter) Less(i, j int) bool {
	p1, p2 := ms.pods[i], ms.pods[j]
	var k int
	for k = 0; k < len(ms.cmp)-1; k++ {
		cmpResult := ms.cmp[k](p1, p2)
		// p1 is less than p2
		if cmpResult < 0 {
			return true
		}
		// p1 is greater than p2
		if cmpResult > 0 {
			return false
		}
		// we don't know yet
	}
	// the last cmp func is the final decider
	return ms.cmp[k](p1, p2) < 0
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (ms *multiSorter) Sort(pods []*v1.Pod) {
	ms.pods = pods
	sort.Sort(ms)
}
