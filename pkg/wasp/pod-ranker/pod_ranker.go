package pod_ranker

import (
	v1 "k8s.io/api/core/v1"
	"strings"
)

// PodRanker is an interface for ranking pods
type PodRanker interface {
	RankPods(pods []v1.Pod) []v1.Pod
}

type PodRankerImpl struct{}

func NewPodRankerImpl() *PodRankerImpl {
	return &PodRankerImpl{}
}

// RankPods dummy implementation to evict pods with memory substring first
func (pr *PodRankerImpl) RankPods(pods []v1.Pod) []v1.Pod {
	var prioritizedPods, otherPods []v1.Pod

	for _, pod := range pods {
		if strings.Contains(pod.Name, "memory") {
			prioritizedPods = append(prioritizedPods, pod)
		} else {
			otherPods = append(otherPods, pod)
		}
	}

	return append(prioritizedPods, otherPods...)
}
