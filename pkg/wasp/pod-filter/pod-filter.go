package pod_filter

import (
	v1 "k8s.io/api/core/v1"
)

// PodFilter is an interface for filtering pods
type PodFilter interface {
	FilterPods(pods []*v1.Pod) []*v1.Pod
}

// PodFilterImpl implements PodFilter interface
type PodFilterImpl struct {
	waspNs string
}

func NewPodFilterImpl(waspNs string) *PodFilterImpl {
	return &PodFilterImpl{
		waspNs: waspNs,
	}
}

func (pr *PodFilterImpl) FilterPods(pods []*v1.Pod) []*v1.Pod {
	var filteredPods []*v1.Pod

	for _, pod := range pods {
		// Check if the namespace name starts with any of the excluded prefixes
		if pod.Namespace != pr.waspNs && !IsCriticalPod(pod) {
			filteredPods = append(filteredPods, pod)
		}
	}

	return filteredPods
}
