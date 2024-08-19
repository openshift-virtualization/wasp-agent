package pod_filter

import (
	"strings"

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

	// Prefixes to exclude
	excludedPrefixesForNamespaces := []string{"openshift", "kube-system"}

	for _, pod := range pods {
		// Check if the namespace name starts with any of the excluded prefixes
		if pod.Namespace != pr.waspNs && !pr.hasExcludedPrefix(pod.Namespace, excludedPrefixesForNamespaces) && !IsCriticalPod(pod) {
			filteredPods = append(filteredPods, pod)
		}
	}

	return filteredPods
}

// hasExcludedPrefix checks if the given namespace name starts with any of the excluded prefixes
func (pr *PodFilterImpl) hasExcludedPrefix(namespace string, excludedPrefixes []string) bool {
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(namespace, prefix) {
			return true
		}
	}
	return false
}
