package pod_ranker

import (
	stats_collector "github.com/openshift-virtualization/wasp-agent/pkg/wasp/stats-collector"
	v1 "k8s.io/api/core/v1"
	v1resource "k8s.io/kubernetes/pkg/api/v1/resource"
)

// PodRanker is an interface for ranking pods
type PodRanker interface {
	RankPods(pods []*v1.Pod) error
}

type PodRankerImpl struct {
	podStatsCollector stats_collector.PodStatsCollector
}

func NewPodRankerImpl(podStatsCollector stats_collector.PodStatsCollector) *PodRankerImpl {
	return &PodRankerImpl{podStatsCollector}
}

// RankPods dummy implementation to evict pods with memory substring first
func (pr *PodRankerImpl) RankPods(pods []*v1.Pod) error {
	podSummary, err := pr.podStatsCollector.ListPodsSummary()
	if err != nil {
		return err
	}
	summary := func(pod *v1.Pod) (stats_collector.PodSummary, bool) {
		for _, ps := range podSummary {
			if ps.UID == pod.UID && ps.MemoryWorkingSetBytes != nil && ps.MemorySwapCurrentBytes != nil {
				return ps, true
			}
		}
		return stats_collector.PodSummary{}, false
	}
	orderedBy(exceedMemory(summary, GetResourceLimitsQuantity), exceedMemory(summary, v1resource.GetResourceRequestQuantity), priority, memory(summary)).Sort(pods)
	return nil
}
